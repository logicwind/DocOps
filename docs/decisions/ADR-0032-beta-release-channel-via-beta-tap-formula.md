---
title: Beta release channel via @beta tap-formula
status: accepted
coverage: required
date: "2026-05-04"
supersedes: []
related: [ADR-0019, ADR-0012]
tags: []
---

## Context

The release pipeline (`.goreleaser.yml`, `logicwind/homebrew-tap`,
`logicwind/scoop-bucket`) currently publishes one channel: stable. Tag a
SemVer like `v0.6.0`, GoReleaser builds binaries, bumps the brew + scoop
formulas, GitHub Release goes out. Users run
`brew install logicwind/tap/docops` and always get the latest stable.

We don't yet have a way to ship pre-1.0 prereleases (`v0.7.0-beta.1`,
`-alpha.1`, `-rc.1`) to opt-in testers without disturbing stable users.
Two patterns were on the table:

- **A. `skip_upload: auto`** — tag prereleases as SemVer prereleases,
  GoReleaser skips the formula bump. Stable brew users are protected,
  but prereleases are *not installable via brew*; testers must download
  from the GitHub release page or curl a tarball.
- **B. Second `brews:` entry under the same tap, named `docops@beta`** —
  prereleases bump only the `docops@beta.rb` formula via templated
  `skip_upload`. Testers run `brew install logicwind/tap/docops@beta` —
  one command, no tap reconfiguration. Stable formula stays untouched.

Option B is the standard convention (`node@20`, `postgresql@16`,
`gh@beta`). Scoop has no `@` convention; the parallel name is
`docops-beta` in the same bucket.

## Decision

When we need a beta channel, ship it as **Option B**: a second
`brews:` entry named `docops@beta` and a second `scoops:` entry named
`docops-beta`, both pushing to the existing
`logicwind/homebrew-tap` and `logicwind/scoop-bucket` repos. Channel
selection is driven by templated `skip_upload`:

```yaml
brews:
  - name: docops
    skip_upload: '{{ if .Prerelease }}true{{ else }}false{{ end }}'
    # ...stable settings unchanged
  - name: docops@beta
    skip_upload: '{{ if .Prerelease }}false{{ else }}true{{ end }}'
    # ...same repository / token / description
```

Tag convention: SemVer prereleases (`vX.Y.Z-beta.N`, `-alpha.N`,
`-rc.N`). GoReleaser's prerelease detector is what flips
`.Prerelease` to non-empty and routes the formula write.

**Not now.** This ADR is `draft` because we have no testers asking for
a beta channel yet. Releases through at least v1.0 stay
single-channel (stable only). Implementation lands when there's a
concrete tester signal — see TP.

## Rationale

- **Standard Homebrew convention.** `name@channel` is what brew users
  already recognise; no docs gymnastics, `brew search` shows both.
- **One tap, two formulas.** No second `homebrew-tap-beta` repo to
  maintain or token-wire; reuses the migration we just finished
  (TP-024).
- **Stable users never change behaviour.** Templated `skip_upload`
  gates writes per-tag; a stable release leaves `docops@beta.rb`
  alone, a prerelease leaves `docops.rb` alone.
- **Cheap to defer.** Adding it is ~15 lines of `.goreleaser.yml` plus
  a CHANGELOG note; the cost of waiting is zero, the cost of shipping
  before there are testers is a formula nobody installs.
- **Option A rejected** because it offers prerelease *binaries* but no
  brew install path — testers fall back to curl, which is a worse
  experience than just waiting for stable.

## Consequences

- `.goreleaser.yml` gains a second `brews:` block and a second
  `scoops:` block when the channel ships.
- Tag namespace `vX.Y.Z-(alpha|beta|rc).N` becomes load-bearing; CI
  release workflow already passes prerelease tags through unchanged.
- Docs (README install snippet, `docops.dev` if/when it exists) need
  to mention the `@beta` formula only after it's live.
- Scoop beta name is `docops-beta` (no `@`), worth calling out to
  avoid users guessing `docops@beta` on Windows.
- This ADR stays `draft` until the TP moves to `active`. When the TP
  ships, flip this ADR to `accepted` in the same change.
