---
title: docops refresh — composite validate+index+state for the edit loop
status: backlog
priority: p1
assignee: unassigned
requires: [ADR-0020]
depends_on: [TP-003, TP-004, TP-005]
---

# docops refresh — composite validate+index+state for the edit loop

## Goal

Collapse the three-command chain agents and humans run after every
doc edit (`validate && index && state`) into a single `docops refresh`
subcommand. `audit` stays separate — it is advisory, not a refresh
step.

## Acceptance

- New subcommand `docops refresh` wired in `cmd/docops/main.go`.
- Implementation in `cmd/docops/cmd_refresh.go` — thin orchestrator
  that calls the existing validator/index/state packages in order.
  No new logic in `internal/`; this is pure composition.
- Execution order: validate → index → state. Stop at the first error
  (validate failing means the tree cannot be indexed safely; same for
  index failing before state).
- Human output, one line per step, prefixed with the step name and
  `OK` / `FAIL`:
  ```
  validate: OK (34 docs, 0 errors, 0 warnings)
  index:    OK (wrote docs/.index.json)
  state:    OK (wrote docs/STATE.md)
  docops refresh: OK
  ```
- `--json` flag emits an aggregate shape:
  ```json
  {
    "ok": true,
    "steps": [
      {"name": "validate", "ok": true, "errors": 0, "warnings": 0, "files": 34},
      {"name": "index",    "ok": true, "path": "docs/.index.json",  "docs": 34},
      {"name": "state",    "ok": true, "path": "docs/STATE.md"}
    ]
  }
  ```
- Exit codes:
  - 0 all steps OK.
  - 1 validate failed (at least one error) → index / state are
    **not** run; their step entry is present with `"skipped": true`.
  - 2 bootstrap error (no docops.yaml, etc.) — matches every other
    command.
- Update `cmd/docops/main.go` top-level usage to list `refresh` in
  the command table. Remove it from the "coming" list if present
  (it is not, but double-check).
- Update the root `AGENTS.md` bootstrap-state list to include
  `refresh` in the shipped commands.
- Update `templates/AGENTS.md.tmpl` to mention `docops refresh` in
  the CLI section so user repos learn about it on init.
- Add a `refresh.md` skill to `templates/skills/docops/` using the
  `/docops:refresh` naming convention. Short (10–20 lines): run after
  any doc edit; `--json` for CI.
- Tests in `cmd/docops/cmd_refresh_test.go` covering: happy path,
  validate-failure-short-circuits-index, JSON shape, exit codes.
- Full test suite passes with `go test -race ./...` and
  `go vet ./...`.

## Notes

Do not change the pre-commit hook in this task. The hook decision
(validate-only vs refresh-on-commit) is explicitly deferred in
ADR-0020 — auto-regenerating committed artifacts from a hook creates
staging awkwardness that is worse than today's manual call.

Do not pipe per-command output through `refresh` — the individual
commands already print what they need on their own writers. Refresh
prints its one-line-per-step summary to stdout and relies on the
underlying commands to write their own side effects (the schema file,
the index file, etc.).

Do not regress any existing validate/index/state behaviour. The
three commands remain callable individually; `refresh` is additive.
