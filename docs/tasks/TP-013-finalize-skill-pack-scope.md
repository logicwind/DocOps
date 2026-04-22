---
title: Finalize skill-pack scope for v0.1.0 — close TP-010 gap
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0019, ADR-0013]
depends_on: [TP-007]
---

# Finalize skill-pack scope for v0.1.0 — close TP-010 gap

## Goal

ADR-0019 rescopes the skill-pack work: init-based distribution is the
single path for v0.1.0; the `packages/` standalone bundles are
deferred. This task closes the remaining gap items from TP-010 that
are still in scope, so TP-010 can be marked done.

## Acceptance

- `docops init --no-skills` scaffolds everything except
  `.claude/skills/docops/` and `.cursor/commands/docops/`. Existing
  skill files are left untouched when the flag is set (they are not
  deleted).
- CI lint script (`scripts/lint-skills.sh` or a Go test under
  `internal/templates` — pick what fits best) verifies every code
  fence in `templates/skills/docops/*.md` that starts with `docops `
  invokes a known subcommand and known flags. The list of known
  subcommands is `init|validate|index|state|audit|new|schema`; unknown
  names fail the lint. Runs in `make lint` or as a `go test` target.
- Skills that reference `docops-review` or `docops-graph` are removed
  from `templates/skills/docops/` (they shouldn't exist — verify and
  keep the templates free of skills that wrap unshipped commands).
- `templates/skills/docops/new-adr.md`, `new-task.md`, `new-ctx.md`
  reference the real flags of the shipped `docops new` command
  (`--requires`, `--type`, `--related`, `--priority`, `--assignee`,
  `--no-open`, `--json`). Any that reference flags the CLI does not
  have get fixed.
- TP-010's task file is marked `done` after this task lands, with a
  short note pointing at ADR-0019 and TP-013 as the realized scope.

## Notes

Keep the lint minimal — shelling out to `docops <cmd> --help` and
asserting exit 0 is acceptable, but faster is a static parse of the
template bodies against a hardcoded allowlist of subcommands and flags.
Don't introduce a dependency on the binary being on `PATH` during
`go test` — use the allowlist form.

The skills already use `/docops:<name>` naming (`init`, `state`,
`new-task`, …); that naming stays. The lint only checks the CLI
invocations shown inside fenced code blocks, not the skill headings.
