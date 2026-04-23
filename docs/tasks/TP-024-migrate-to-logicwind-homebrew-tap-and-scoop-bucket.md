---
title: Migrate tap/bucket to logicwind/homebrew-tap + logicwind/scoop-bucket and enable goreleaser publishing
status: active
priority: p1
assignee: nix
requires: [ADR-0019, ADR-0012]
depends_on: []
---

> **2026-04-23 progress note.** Phase 1 (new repos created) ✅.
> Phase 3 (legacy repos archived) ✅. `.goreleaser.yml` has
> `skip_upload: false`; `.github/workflows/release.yml` passes
> `HOMEBREW_TAP_GITHUB_TOKEN` (aliased to `SCOOP_TAP_GITHUB_TOKEN`)
> to goreleaser; the PAT secret is provisioned. **Only remaining
> step:** cut `v0.2.1` via `make release VERSION=0.2.1` on a clean
> main. CI will populate both new repos on first run.

# Migrate tap/bucket to logicwind/homebrew-tap + logicwind/scoop-bucket and enable goreleaser publishing

## Goal

Two things, done together:

1. **Rename the tap/bucket repos** from the per-tool pattern
   (`logicwind/homebrew-docops`, `logicwind/scoop-docops`) to the
   org-wide pattern (`logicwind/homebrew-tap`, `logicwind/scoop-bucket`).
   Install becomes `brew install logicwind/tap/docops` — matches the
   Vercel / HashiCorp / Fly.io convention and future-proofs for other
   Logicwind CLIs.
2. **Enable goreleaser auto-publish** so every tagged release pushes
   the formula/manifest automatically. No more hand-authoring.

The decision to flip now (2026-04-23) is gated on **pre-launch**:
no external users are installing DocOps yet, so the one-time
install-path break is free.

## Current state

- `logicwind/homebrew-docops` — exists, published 2026-04-22 with
  hand-authored formulas for v0.1.0 and v0.1.1.
- `logicwind/scoop-docops` — exists, published 2026-04-22 with
  manifests for v0.1.0 and v0.1.1.
- `.goreleaser.yml` — already targets the new names
  (`homebrew-tap`, `scoop-bucket`) with `skip_upload: true`
  preventing any premature push.
- `README.md` — already advertises `brew install logicwind/tap/docops`
  and the scoop `logicwind` bucket. Those commands **do not work
  yet** — the new repos don't exist.
- ADR-0019's amendment log records the naming change via the
  ADR-0025 amendment machinery (or a pre-schema HTML stub until
  TP-026 lands).

## Plan

### Phase 1 — Create new repos and seed with current formula

```sh
# Both public; substitute description if you want finer wording.
gh repo create logicwind/homebrew-tap \
  --public \
  --description "Official Homebrew tap for Logicwind CLIs"
gh repo create logicwind/scoop-bucket \
  --public \
  --description "Official Scoop bucket for Logicwind CLIs"

# Copy v0.1.1 formula + manifest into the new repos so
# `brew install logicwind/tap/docops` works immediately.
# Pick either approach:
#   (a) clone both old repos, copy Formula/ and bucket/ trees
#       into freshly cloned new repos, commit + push.
#   (b) cut v0.2.1 from source after Phase 2 below with
#       auto-publish enabled; skip the manual seeding.
# (b) is cleaner if you're ready to cut v0.2.1 soon.
```

### Phase 2 — Enable goreleaser auto-publish

1. Provision a release token (PAT or GitHub App) with `repo` scope on
   `logicwind/homebrew-tap` and `logicwind/scoop-bucket` (and only
   those — minimum-privilege).
2. Remove `skip_upload: true` from both blocks in `.goreleaser.yml`.
3. Wire the token into `.github/workflows/release.yml` as an env var
   (`HOMEBREW_TAP_GITHUB_TOKEN` or the goreleaser-default name for
   your goreleaser version — v2 defaults shifted).
4. Cut a test tag (e.g. `v0.2.1`) and confirm the workflow publishes
   both the formula and the manifest.

### Phase 3 — Retire the legacy per-tool repos

Only after Phase 1 + 2 are verified green:

```sh
# Add a "moved to" README at the top of each legacy repo
# (one commit each) pointing at the new tap/bucket.
# Then archive.
gh repo archive logicwind/homebrew-docops --yes
gh repo archive logicwind/scoop-docops --yes
```

Archiving, not deleting — preserves the v0.1.0 + v0.1.1 formula git
history and the install paths stay resolvable (if anyone actually
pinned to them, they at least see an archived banner).

If your dev machine was running
`brew install logicwind/docops/docops`:

```sh
brew uninstall docops 2>/dev/null || true
brew untap logicwind/docops 2>/dev/null || true
brew install logicwind/tap/docops
```

## Acceptance

- `logicwind/homebrew-tap` and `logicwind/scoop-bucket` exist, public,
  with at least one formula/manifest for the current release.
- `.goreleaser.yml` brew/scoop blocks no longer carry `skip_upload: true`.
- `.github/workflows/release.yml` exposes the tap token to goreleaser.
- A tag triggers goreleaser, which pushes:
  - `Formula/docops.rb` → `homebrew-tap`
  - `bucket/docops.json` → `scoop-bucket`
- `brew install logicwind/tap/docops` and
  `scoop bucket add logicwind https://github.com/logicwind/scoop-bucket && scoop install docops`
  both return the current version on a fresh machine.
- Legacy repos `logicwind/homebrew-docops` and `logicwind/scoop-docops`
  are archived with a "moved to" README pointing at the new home.

## Notes

- **Token mechanism:** PAT is the simple path; a GitHub App is the
  long-term-correct path. Start with PAT — revisit if the org
  standardises on Apps later.
- If a publisher fails partway, the release succeeds partially
  (binaries upload, tap push fails). Annoying but not destructive.
  Goreleaser's error output identifies the failing publisher.
- **Out of scope for this task:** Chocolatey, Docker image publishing
  (ADR-0012 mentions both; both can be follow-ups).
- **Cannot be done by an agent unattended** — repo creation, token
  provisioning, and archive need a human with write access to the
  `logicwind` GitHub org. Document the human-step list in the PR.
- If anyone (even a teammate's dev machine) has `brew tap
  logicwind/docops` locally, they will need the un-tap / re-tap
  sequence shown above. Pre-launch: likely just your machine.
