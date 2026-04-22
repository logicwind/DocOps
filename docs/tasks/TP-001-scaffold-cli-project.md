---
title: Scaffold the @docops/core CLI project
status: done
priority: p0
assignee: claude
requires: [ADR-0012, ADR-0011]
depends_on: []
---

# Scaffold the @docops/core CLI project

## Goal

Create the initial project structure for the DocOps CLI: source tree, build pipeline, cross-compile configuration, release automation. Produce a "hello world" binary that prints a version and exits.

## Implementation language decision

Pick between Bun-compiled TypeScript and Go. Document the choice in a short appendix to ADR-0012 or a follow-up ADR. Criteria:

- Binary size (target < 30 MB).
- Cross-compile ease for darwin-arm64, darwin-x64, linux-x64, linux-arm64, windows-x64.
- Developer velocity for the author.
- Ecosystem fit for YAML parsing, markdown parsing, JSON Schema emission.

## Acceptance

- Repo layout with `src/`, `bin/`, `Taskfile`/`Makefile` or equivalent.
- `docops --version` produces output.
- CI matrix builds all target platforms on every commit.
- GitHub Releases workflow attaches binaries to tags.
- README documents install paths (direct download, brew formula stub, scoop manifest stub).
- All dev dependencies pinned.

## Notes

This task unblocks everything else. Do not over-engineer — minimal skeleton, working build, working release. Schemas and commands land in subsequent tasks.

## Outcome (2026-04-22)

- **Language:** Go — rationale appended to ADR-0012.
- **Layout:** `cmd/docops/main.go`, `internal/version/`, `Makefile`, `.goreleaser.yml`, `.github/workflows/{ci,release}.yml`, `README.md`, `.gitignore`.
- **Binary size:** 1.8 MB (darwin-arm64, `-s -w`) — 6% of the 30 MB target.
- **Release tooling:** `goreleaser` emits per-platform archives + checksums + Homebrew formula stub + Scoop manifest stub. Tap and bucket repos not yet created (marked `skip_upload: auto` until they exist).
- **CI:** test on ubuntu/macos/windows; cross-compile matrix for darwin-amd64, darwin-arm64, linux-amd64, linux-arm64, windows-amd64; 30 MB size gate on every build.
- **Deferred:** npm per-platform publish (`optionalDependencies` pattern documented in ADR-0012 addendum — separate task when we cut v0.1.0), Docker image publish, LICENSE file.
