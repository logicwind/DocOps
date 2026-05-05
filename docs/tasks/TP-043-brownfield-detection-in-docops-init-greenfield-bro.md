---
title: Brownfield detection in docops init + greenfield/brownfield closing message
status: done
priority: p2
assignee: claude
requires: [ADR-0033]
depends_on: []
---

## Goal

Make `docops init` end with an actionable next-step block tailored to
whether the repo is brownfield (existing code) or greenfield (empty).

## Scope

1. **Detector** in `internal/cli/init.go` (or a new
   `internal/scaffold/detect.go` if cleaner):
   - Brownfield if any of these exist at the working dir:
     `package.json`, `go.mod`, `Cargo.toml`, `pyproject.toml`,
     `Gemfile`, `pom.xml`, `composer.json`, `requirements.txt`,
     `src/`, `app/`, `lib/`.
   - Brownfield also if `git rev-list --count HEAD` returns > 10.
   - Otherwise greenfield.
   - Pure file/git inspection. No network. <50ms.
2. **Routing** the closing message:
   - Greenfield: suggest `docops new ctx --type brief "..."` and
     `/docops:plan`.
   - Brownfield: suggest `/docops:onboard` first, then
     `docops new ctx` as the manual fallback. Mention which signals
     fired (e.g. "Detected: go.mod, src/, 184 commits").
3. **`--json` output** of `docops init` gains a `next_steps: []`
   array of `{label, command}` objects so programmatic callers can
   render their own affordances. Detection result also surfaces as
   `mode: "brownfield" | "greenfield"`.
4. **Tests**:
   - Unit tests on the detector across fixture dirs (empty, only
     git, only `package.json`, only `src/`, mixed).
   - Golden test on the closing message for both modes.

## Out of scope

- Reading the contents of any detected file. Detection is name/path
  only.
- Suppressing the block via `--quiet` — handled by the broader
  affordances task (TP for §3 of ADR-0033).

## Done when

- `docops init` in an empty dir prints the greenfield block.
- `docops init` in a repo with `go.mod` (or any trigger) prints the
  brownfield block, naming the detected signals.
- `docops init --json` includes `mode` and `next_steps[]`.
- Unit + golden tests pass.
