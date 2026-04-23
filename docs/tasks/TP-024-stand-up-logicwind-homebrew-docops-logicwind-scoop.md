---
title: Stand up logicwind/homebrew-docops + logicwind/scoop-docops taps and enable goreleaser publishing
status: backlog
priority: p1
assignee: unassigned
requires: [ADR-0019, ADR-0012]
depends_on: []
---

# Stand up logicwind/homebrew-docops + logicwind/scoop-docops taps and enable goreleaser publishing

## Goal

Make `brew install logicwind/docops/docops` and
`scoop install docops` work for v0.2.x onward. ADR-0019 deferred this
to a follow-up release; goreleaser already has the brew/scoop blocks
configured with `skip_upload: true` (`.goreleaser.yaml`). This task
flips that switch by:

1. Creating the two empty GitHub repos under the `logicwind` org.
2. Provisioning a release token with `repo` scope on those two repos
   (and only those — minimum-privilege).
3. Removing `skip_upload: true` from `.goreleaser.yaml`.
4. Wiring the token into `.github/workflows/release.yml` as an env
   var (`HOMEBREW_TAP_GITHUB_TOKEN` or the goreleaser-default name).
5. Cutting v0.2.1 to verify the publishers run end-to-end.

This is currently advertised in the README install snippets that
**don't actually work** — making it a user-facing breakage on v0.2.0,
not new feature work. Hence p1.

## Acceptance

- `https://github.com/logicwind/homebrew-docops` exists, public,
  empty initial commit. README points at logicwind/DocOps.
- `https://github.com/logicwind/scoop-docops` exists, same shape.
- `.goreleaser.yaml` brew/scoop blocks no longer carry
  `skip_upload: true`.
- `.github/workflows/release.yml` exposes the tap token to goreleaser.
- A v0.2.1 release tag triggers goreleaser, which:
  - Pushes a `Formula/docops.rb` commit to `homebrew-docops`.
  - Pushes a `bucket/docops.json` commit to `scoop-docops`.
- After release: `brew tap logicwind/docops && brew install docops`
  works on a fresh macOS box; `scoop bucket add docops https://github.com/logicwind/scoop-docops && scoop install docops`
  works on a fresh Windows box.
- README "Installation" section updated: the tap/bucket commands
  documented today are now real instructions, not placeholders.

## Notes

- **Token mechanism:** PAT is the simple path; a GitHub App is the
  long-term-correct path. Start with PAT scoped to the two tap repos
  only — revisit if the org standardises on Apps later.
- The release.yml changes are tiny — one `env:` block under the
  goreleaser action. Token name follows goreleaser conventions
  (check goreleaser docs at task-start time; defaults shifted in v2).
- `scoops:` block in `.goreleaser.yaml` already names the repo
  `scoop-docops`; if we choose a different convention later
  (e.g. `scoop-bucket`) update both the repo name and the block.
- Test plan: cut v0.2.1 *only after* the two repos exist and the
  token is in place. If the publishers fail, the release succeeds
  partially (binaries upload, tap push fails) — annoying but not
  destructive. Goreleaser's error output identifies the failing
  publisher precisely.
- **Out of scope for this task:** Chocolatey, Docker image
  publishing (ADR-0012 mentions both; both can be follow-ups).
- **Cannot be done by an agent unattended** — repo creation and
  token provisioning need a human with write access to the
  `logicwind` GitHub org. Document the human-step list in the PR.
