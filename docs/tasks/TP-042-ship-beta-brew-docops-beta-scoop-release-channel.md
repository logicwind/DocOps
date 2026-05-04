---
title: Ship @beta brew + docops-beta scoop release channel
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0032]
depends_on: []
---

## Goal

Add the opt-in beta channel described in ADR-0032 so testers can run
`brew install logicwind/tap/docops@beta` (and `scoop install
docops-beta`) without disturbing stable users.

**Do not start until there is a concrete tester ask.** This task is
backlog by design — see the "Not now" rationale in ADR-0032.

## Scope

1. `.goreleaser.yml`:
   - Add `skip_upload: '{{ if .Prerelease }}true{{ else }}false{{ end }}'`
     to the existing `brews:` and `scoops:` entries.
   - Add a second `brews:` entry `name: docops@beta` with the inverse
     `skip_upload` template, same `repository`, `token`,
     `description`, `homepage`, `license`, `test`.
   - Add a second `scoops:` entry `name: docops-beta` with the same
     pattern.
2. CHANGELOG: note the new install command under the release that
   ships it.
3. README install section: add a short "Beta channel" subsection with
   the brew + scoop commands.
4. First prerelease tag (e.g. `vX.Y.Z-beta.1`) to validate the path
   end-to-end. Confirm:
   - GitHub release shows the "Pre-release" badge.
   - `homebrew-tap` receives only `Formula/docops@beta.rb`.
   - `scoop-bucket` receives only `bucket/docops-beta.json`.
   - Stable formula files are untouched on the same release run.

## Out of scope

- Separate `homebrew-tap-beta` repo (rejected in ADR-0032).
- Promotion automation between beta and stable.
- Versioned channels beyond `@beta` (no `@next`, `@canary`).

## Done when

- A prerelease tag round-trips through CI and produces installable
  `docops@beta` / `docops-beta` artifacts.
- README + CHANGELOG mention the channel.
- Flip ADR-0032 from `draft` to `accepted` in the same PR.
