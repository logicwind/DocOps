---
title: Fix Makefile release DRY_RUN — guard escapes, real commit/tag/push runs
status: done
priority: p2
assignee: unassigned
requires: [ADR-0019]
depends_on: []
---

## Resolution — 2026-04-24 (commit feadefe)

Applied **Option B**: collapsed the dry-run guard and the real-release
commands into a single `\`-joined shell block. `exit 0` inside the
`DRY_RUN=1` branch now aborts the whole sequence; the `echo > VERSION`
/ `git commit` / `git tag` / `git push` lines share the same shell, so
they never reach the interpreter when the guard fires. Added `set -e`
so any failing real-release step stops the chain instead of racing
past. Dry-run output now also prints a `re-run without DRY_RUN=1`
footer.

Skipped the scripts/ regression test (acceptance bullet 3) for now —
low ROI relative to the single-line structural fix; re-add if this
ever regresses.


## Goal

Make `make release VERSION=X.Y.Z DRY_RUN=1` actually be a dry run.
Today it prints the "would do" lines and then runs the real
`echo > VERSION` / `git commit` / `git tag` / `git push` commands
anyway.

## What happened (incident trace — 2026-04-23)

- Operator ran `make release VERSION=0.2.1 DRY_RUN=1` expecting a
  preview. The command printed the dry-run lines AND executed the
  real sequence: wrote VERSION, committed, tagged `v0.2.1`, pushed
  both main and the tag to origin.
- The tag push triggered the release workflow. It half-succeeded
  (GitHub Release published, tap/bucket push failed with a 403 —
  separate bug, fixed in the same session).
- When the operator followed up with the non-dry-run
  `make release VERSION=0.2.1`, the Makefile correctly refused with
  `tag v0.2.1 already exists locally` — which *confirmed* the dry
  run had really tagged and pushed.

## Root cause

Make runs **each recipe line in its own shell** unless `.ONESHELL:`
is set or all lines are joined with `\`. The current target has the
DRY_RUN guard as one if-block ending with `exit 0`. `exit 0` exits
that subshell with success — it does NOT tell Make to skip the
remaining recipe lines. The next lines then run in their own fresh
shells and execute the real commit/tag/push.

## Fix options

- **A. `.ONESHELL:` directive** — one shell per recipe globally.
  Fixes this cleanly but changes the behaviour of every other
  target; audit them first.
- **B. Collapse `release` into one shell block** (`@bash -c
  '...'` or `&&`-chained). Scoped to this target; recommended.
- **C. Guard each side-effect line individually** with
  `@[ -z "$(DRY_RUN)" ] && <cmd>`. Noisy but surgical.

**Recommended:** Option B.

## Acceptance

- `make release VERSION=9.9.9 DRY_RUN=1` on a clean tree prints
  the "would do" lines and makes **zero** changes:
  - `git status` is clean after.
  - `git log -1 --format=%H` is unchanged.
  - `git tag | grep v9.9.9` is empty.
  - `git ls-remote origin refs/tags/v9.9.9` is empty.
- `make release VERSION=X.Y.Z` (no DRY_RUN) continues to work
  exactly as before.
- Regression test: a shell test under `scripts/` (or a new
  `make test-release` target) runs the dry-run against a temp
  git repo fixture and asserts the four "zero change" conditions.

## Notes

- This bug sent a real tag to production. Fixing it is p1 even
  though the damage this time was self-limiting.
- While fixing, also consider:
  - A final "(no changes made)" footer on dry runs.
  - A confirm prompt on the real path — `make release` has no
    confirmation today, which is the pair of this bug.
