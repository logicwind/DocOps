---
title: Release runbook — stable and beta channels
type: notes
supersedes: []
---

## Purpose

Operational runbook for cutting docops releases. Two channels: **stable** (`brew install logicwind/tap/docops`) and **beta** (`brew install logicwind/tap/docops@beta`). Beta is the dogfood channel — bake a candidate there for a few days, then promote to stable.

See [ADR-0019](../decisions/ADR-0019-ship-v0-1-0-from-logicwind-docops.md) for the original release pipeline, [ADR-0032](../decisions/ADR-0032-beta-release-channel-via-beta-tap-formula.md) for the beta channel design, and TP-042 for the implementation.

## Two loops

| Loop | When | Speed |
|---|---|---|
| `make install` | tweaking source; testing in another project on this machine | seconds |
| `make beta` | candidate is good enough to live on for a few days | ~3–5 min (CI + brew) |
| `make release` | beta has held up; promote to stable | ~3–5 min (CI + brew) |

The fast loop has no tag, no CI, no brew. `go install` puts your working tree's `docops` into `$GOPATH/bin`, which is on PATH ahead of brew. The slow loops actually publish artifacts.

## Day-to-day tweaking (fast loop)

```sh
make install                           # builds working tree → $GOPATH/bin/docops
cd ~/some-project && docops <whatever> # exercise it in a real repo
```

No tag, no public artifact. Iterate until the change feels right.

## Cut a beta (dogfood loop)

From any branch — `dev`, `staging`, or a feature branch:

```sh
make beta VERSION=0.6.1-beta.1 DRY_RUN=1   # preview tag/push
make beta VERSION=0.6.1-beta.1             # tag + push
gh run watch                                # tail the release workflow
brew upgrade logicwind/tap/docops@beta      # pull the fresh beta
```

What `make beta` does:

1. Validates the version matches `X.Y.Z-(alpha|beta|rc).N`.
2. Refuses to run with a dirty tree or a duplicate tag.
3. Creates an annotated tag `v$VERSION` (does **not** bump the `VERSION` file — that file tracks latest stable).
4. Pushes the tag. The push triggers `.github/workflows/release.yml`, goreleaser sees `.Prerelease="beta.N"`, bumps **only** `Formula/docops@beta.rb` and `bucket/docops-beta.json`. Stable formula files are untouched. The GitHub release shows the "Pre-release" badge.

To cut another beta on the same target version, increment N: `make beta VERSION=0.6.1-beta.2`.

## Promote to stable

Once the beta has held up:

```sh
git checkout main && git pull
make release VERSION=0.6.1                 # bumps VERSION, commits, tags, pushes
gh run watch
```

`make release` enforces:

- Branch must be `main`.
- Tree must be clean.
- Tag must not already exist (locally or on origin).
- Tag pushed must equal the new `VERSION` file content (CI re-checks).

Goreleaser then bumps `Formula/docops.rb` and `bucket/docops.json`. The `@beta` formula stays untouched until the next prerelease tag.

## GitHub release notes

GoReleaser auto-generates release notes from commit messages between tags, filtered to exclude `docs:`/`test:`/`chore:` (see `changelog:` in `.goreleaser.yml`). On top of that, the `release.header` template (also in `.goreleaser.yml`) prepends a link to `CHANGELOG.md` and the README install section.

For the human-written narrative, update `CHANGELOG.md` **before** tagging — the auto-notes are commit summaries, the CHANGELOG is the story.

## Pre-flight checklist

Before `make release` (stable):

- [ ] On `main`, clean tree.
- [ ] `CHANGELOG.md` has an entry for the new version (move the `Unreleased` block under a dated heading).
- [ ] `make test` and `make lint` are green.
- [ ] At least one beta has been cut and used on a real project for ≥1 working day.

Before `make beta`:

- [ ] Clean tree, branch doesn't matter.
- [ ] Pick the next prerelease number (`-beta.1` if first on this target, otherwise increment).

## Recovery

- **Tag pushed by mistake.** Delete locally and on origin: `git tag -d vX.Y.Z && git push --delete origin vX.Y.Z`. If the release workflow already published, also delete the GitHub release via `gh release delete vX.Y.Z`. Tap/bucket commits will need a manual revert PR — they are separate repos.
- **Goreleaser fails mid-run.** Re-running is generally safe; tap/bucket commits are idempotent on the same tag. Inspect `gh run view --log-failed`.
- **VERSION mismatch on stable tag.** CI fails fast with a clear error. Retag after fixing the `VERSION` file.
