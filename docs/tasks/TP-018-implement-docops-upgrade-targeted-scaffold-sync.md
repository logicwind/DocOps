---
title: Implement docops upgrade — targeted scaffold sync
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0021, ADR-0023]
depends_on: [TP-007, TP-009, TP-013, TP-020, TP-021]
---

# Implement docops upgrade — targeted scaffold sync

## Goal

Ship the `docops upgrade` subcommand that pulls the current binary's
shipped templates into an already-initialized project without
clobbering `docops.yaml` or the pre-commit hook, per ADR-0021.

## Acceptance

- New subcommand `docops upgrade` wired in `cmd/docops/main.go` and
  listed in the top-level help.
- Implementation lives in a new `internal/upgrader/` package whose
  surface mirrors `internal/initter`: an `Options` struct, a `Run`
  entry point that returns an action list, no prompts (prompts live
  in `cmd/docops/cmd_upgrade.go`, same split as TP-015).
- **What upgrade touches by default**:
  - `.claude/skills/docops/*.md` — sync to the shipped bundle. New
    files are created, changed files are overwritten, and files in
    the directory that are not in the shipped bundle are deleted.
  - `.cursor/commands/docops/*.md` — same policy.
  - `docs/.docops/schema/*.schema.json` — regenerate from the loaded
    `docops.yaml` (picks up project `context_types`).
  - `AGENTS.md` — refresh the `<!-- docops:start --> … <!-- docops:end -->`
    block in place, preserving content outside the markers. Reuse
    the merge helper from `internal/initter`.
- **What upgrade does NOT touch by default** (must be covered by a
  test each):
  - `docops.yaml`
  - `.git/hooks/pre-commit`
  - `docs/{context,decisions,tasks}/*.md`
  - `docs/.index.json`, `docs/STATE.md`, `docs/.docops/counters.json`
- **Opt-in flags**:
  - `--config` — also overwrite `docops.yaml` from the shipped template.
  - `--hook` — also reinstall the pre-commit hook.
  - `--dry-run` — print the action plan, write nothing. Exit 0.
  - `--yes` / `-y` — skip the TTY confirm. Mirrors `init`.
- **Refuses without a docops.yaml**: exit 2 with a clear message
  pointing at `docops init`. Matches the bootstrap-error pattern of
  every non-init command.
- **Output shape**: one line per action with a sigil:
  - `+ path  (new)` — file created.
  - `~ path  (refreshed)` — file overwritten.
  - `- path  (removed)` — file deleted because it left the shipped bundle.
  - `= path  (up to date)` — no change.
  - `~ AGENTS.md (block refreshed)` — delimited-block merge.
  Finish with `docops upgrade: applied N change(s), skipped M`.
- `--json` emits `{"ok": true, "actions": [{"path": "…", "kind":
  "new|refreshed|removed|up-to-date|block-refreshed"}, …]}`.
- **Removed-skill deletion is DocOps-scoped**: upgrade reads
  `.claude/skills/docops/` and `.cursor/commands/docops/` and only
  removes files whose **basename** is not in the shipped bundle.
  Files outside those two directories are never touched, even if
  they have `docops` in the name.
- **Tests** in `internal/upgrader/upgrader_test.go`:
  - Fresh run on a v0.1.0-era layout (next.md present, refresh.md
    absent) produces one `+`, several `~`, one `-`, and the AGENTS.md
    block refresh. Idempotent on re-run (all `=`).
  - `docops.yaml` is never touched unless `--config`. Verify via
    file-mtime check or content hash.
  - Pre-commit hook is never touched unless `--hook`.
  - User-created file at `.claude/skills/docops/custom.md` is
    deleted by upgrade (it is inside the DocOps-owned subdir). A
    user-created file at `.claude/skills/my-stuff.md` is NOT touched.
  - `--dry-run` writes nothing; `--yes` skips the prompt path
    (covered by a `cmd/docops/cmd_upgrade_test.go`).
- **Skills lint** (`templates/skills_lint_test.go`) updated to know
  about `upgrade` as a valid subcommand. Add `--config`, `--hook`,
  `--dry-run`, `--yes` as valid flags.
- **Ship a `/docops:upgrade` skill** at
  `templates/skills/docops/upgrade.md` — short, 10–20 lines, matches
  the style of `refresh.md`. Mention it runs after `brew upgrade
  docops` (or equivalent), that it preserves `docops.yaml` by
  default, and that `--config` / `--hook` are opt-in.
- **README**: add a short "Upgrading an existing project" subsection
  under Install, showing:
  ```sh
  brew upgrade docops          # or scoop update docops, etc.
  docops upgrade               # syncs skills, schemas, AGENTS.md block
  docops upgrade --dry-run     # preview first if you prefer
  ```
- **AGENTS.md**: bootstrap-state list grows `upgrade` and the
  "ship as of TP-N" line updates. `templates/AGENTS.md.tmpl` adds a
  one-line mention of `docops upgrade` in the CLI section so future
  init'd projects learn about it.

## Notes

`internal/upgrader/` reuses as much as possible from
`internal/initter/`: the skills-list enumeration, the
`mergeAgentsBlock` helper, the schema emission. Factor shared helpers
into a small `internal/scaffold/` package if that makes the diff
cleaner; otherwise import from `internal/initter` directly.

The removed-skill deletion is the only genuinely new behaviour. Put
a safety belt on it: upgrade refuses to proceed if the
`.claude/skills/docops/` directory contains anything the shipped
bundle did not originate (detected via a stable manifest file
`.claude/skills/docops/.docops-manifest` that upgrade writes alongside
the skills — a simple one-file-per-line list). If the manifest is
absent (v0.1.x user upgrading), write it on first run and treat
everything as docops-owned. Users who want to keep a custom skill
adjacent should put it one level up in `.claude/skills/`.

Do not run `docops refresh` as part of `docops upgrade`. They are
orthogonal: upgrade syncs scaffolding (source-of-truth templates);
refresh regenerates computed artifacts. A user who wants both runs
them in sequence.

Do not auto-tag or auto-release as part of this task. TP-018 is a
backlog item; it ships whenever the next version warrants it, most
likely v0.1.2 or v0.2.0.
