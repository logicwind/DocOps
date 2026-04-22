---
title: docops init — positional [dir], action announcement, and TTY-interactive confirm
status: done
priority: p1
assignee: unassigned
requires: [ADR-0020]
depends_on: [TP-007]
---

# docops init — positional [dir], action announcement, and TTY-interactive confirm

## Goal

Fix the three ergonomic gaps in `docops init` surfaced by v0.1.0
first-user testing: no way to target a different folder, no
explanation of what init will do, no confirmation before writing.

## Acceptance

- `docops init [dir]` accepts an optional positional directory argument:
  - No arg → cwd (current behaviour, no regression).
  - `[dir]` given but missing → create it (and any parent), then init there.
  - `[dir]` given and a non-directory → exit 2 with a clear message.
- Before the plan table, print a short announcement block:
  ```
  docops init will scaffold DocOps in <abs path>:
    - docs/{context,decisions,tasks} folders
    - docops.yaml at the repo root
    - JSON Schemas for editor validation
    - An AGENTS.md block (merges into existing content if present)
    - A .git/hooks/pre-commit hook (if .git exists)
    - /docops:* agent-skill scaffolds under .claude/ and .cursor/
  Safe to re-run; existing files are never silently overwritten.
  ```
  The existing per-action list follows this announcement, unchanged.
- On a TTY, after printing the announcement + plan, prompt
  `Proceed? [y/N]`. `y` or `Y` proceeds. Any other input aborts with
  exit 0 and the message `docops init: aborted by user.` No actions
  executed on abort.
- `--yes` / `-y` flag skips the prompt. `--dry-run` skips the prompt
  (no writes anyway). Non-TTY stdin skips the prompt (CI, piped
  scripts — this is what every mature CLI does).
- Detect TTY via `term.IsTerminal(int(os.Stdin.Fd()))` using
  `golang.org/x/term` (already a transitive dep; add to go.mod if not).
  Do not use a CI env var check — TTY detection is canonical.
- Update `cmd/docops/cmd_init.go` usage string:
  `usage: docops init [dir] [--dry-run] [--force] [--no-skills] [--yes]`.
- Regenerate the `/docops:init` skill template at
  `templates/skills/docops/init.md` to document the new flags and the
  positional arg.
- Tests:
  - `internal/initter` tests stay driven by the package API (no TTY
    dependency); add an `Options.NoConfirm` or equivalent so tests
    skip the confirmation path deterministically. Alternatively the
    initter never prompts — the prompt lives in `cmd/docops/cmd_init.go`
    and the existing initter tests are unaffected. Prefer this
    split — keeps initter test-pure.
  - Add a `cmd/docops/cmd_init_test.go` (or smoke test) that exercises
    the flag parsing: `[dir]` resolution, `--yes` behaviour.
  - Full test suite passes with `go test -race ./...` and `go vet ./...`.

## Notes

Keep the confirmation output compact. Four-line announcement + plan
table + one-line prompt is the budget; anything more feels like a
wizard.

The `[dir]` arg is relative to cwd unless absolute. Resolve to an
absolute path before announcement so the user sees where things will
land.

Do not prompt when the plan is all "skip" — that is the idempotent
re-run case and blocking on y/N there is pure friction. If
`len(changedActions) == 0`, print the plan and exit 0 without
prompting.
