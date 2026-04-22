---
title: Release v0.1.0 under logicwind/DocOps
status: done
priority: p1
assignee: unassigned
requires: [ADR-0019, ADR-0012, ADR-0011]
depends_on: [TP-013]
---

# Release v0.1.0 under logicwind/DocOps

## Goal

Migrate the module path, add the license, tidy the release pipeline,
and cut the first public tag. End state: `github.com/logicwind/DocOps`
exists with `v0.1.0` tagged and goreleaser-produced archives attached.

## Acceptance

- `go.mod` module path is `github.com/logicwind/docops`.
- Every `import "github.com/nachiket/docops/..."` in the Go source is
  rewritten to `github.com/logicwind/docops/...`. `go build ./... &&
  go test ./... -race` passes after the rewrite.
- `LICENSE` file at the repo root: MIT text, copyright
  `Logicwind Technologies Pvt Ltd` (current year).
- `.goreleaser.yml` owner fields updated to `logicwind`; brew tap and
  scoop bucket publisher blocks are present but disabled (e.g.
  `brews: [{ disable: true }]`) so the tag build succeeds without
  those repos existing yet.
- `README.md`:
  - Install snippets switch to `logicwind/docops` URLs (direct download,
    Docker image path, tap/bucket placeholders).
  - The "publishes once v0.1.0 ships" hedging for GitHub Releases is
    removed; the tap/bucket hedging stays until those repos land.
  - The "LICENSE pending" line is removed.
- `templates/AGENTS.md.tmpl` and root `AGENTS.md` mention the release
  channel (GitHub Releases under `logicwind/DocOps`) only where they
  currently reference install paths — no broader rewrite.
- `make release-snapshot` produces a local dist build without error
  against the updated config.
- `docs/STATE.md` and `docs/.index.json` regenerated.

## Notes

Do not run `git tag v0.1.0` or push inside this task — tagging and the
GitHub-repo creation are driven by the human operator after this task
lands clean on `main`. The task's job is to land a tree that is
_ready_ to be tagged.

Module-path rewrite: use `gofmt -r` or a scripted sed pass across `go.mod`
and every `*.go` file. Verify with `grep -rn "nachiket/docops"` returning
zero hits before commit.

The `goreleaser` disabled-publisher state is deliberate (ADR-0019). Do
not enable brew/scoop until the tap and bucket repos exist; that is a
post-v0.1.0 task.

Do not touch `internal/version.Version` default. goreleaser sets it
from the tag at build time via ldflags (see `.goreleaser.yml`); leaving
the in-repo default at `dev` is correct.
