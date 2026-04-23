---
title: Restore /docops:* skills for next, search, get, list, graph commands
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0013, ADR-0019]
depends_on: []
---

# Restore /docops:* skills for next, search, get, list, graph commands

## Goal

Re-add shipped agent skills for the five CLI commands that landed in
TP-011 (`search`) and TP-012 (`get`, `list`, `graph`, `next`) so the
bundle satisfies TP-010/013's policy: a skill exists in
`templates/skills/docops/` iff the corresponding CLI command exists in
the shipped binary.

After v0.2.0 the bundle ships eight skills (`audit`, `init`,
`new-{ctx,adr,task}`, `refresh`, `state`, `upgrade`) but the binary
exposes thirteen non-`new` subcommands. The five missing skills create
silent agent regression — agents that learned to invoke `/docops:next`
under v0.1.x lose that affordance after `docops upgrade` (the
upgrader correctly removed the orphan `next.md` from user repos
because the bundle no longer ships it).

## Acceptance

- `templates/skills/docops/` contains: `next.md`, `search.md`, `get.md`,
  `list.md`, `graph.md` — each with the standard frontmatter (`name`,
  `description`) and a body documenting the command, common flags, and
  typical agent usage. Tone matches the existing skills (terse, points
  at the CLI, doesn't explain DocOps concepts).
- `templates/skills_lint_test.go` knows about each of the five
  subcommands and their flag allowlists (`--json`, kind-specific flags
  for `list`, `--ref` / depth for `graph`, etc.).
- `templates/skills_lint_test.go` passes for every shipped skill.
- A `docops upgrade` run against a v0.2.0-scaffolded repo produces five
  `+ new` lines for the added skills (no removed lines, no other
  refreshes).
- `cmd/docops/cmd_help.go` (or wherever `docops --help` reads from)
  unchanged — this task is skill-only, no CLI surface change.

Patch release v0.2.1 once landed.

## Notes

- The previous `next.md` body (deleted in commit 73cd5db) is a
  reasonable starting point for the new one, but the description
  should match the post-TP-012 reality (depends_on awareness, JSON
  output, etc.).
- `get`/`list` skills should explicitly call out that they're the
  agent-friendly read commands and that `docops state` /
  `docs/STATE.md` remain the right entry points for orientation.
- `graph` should mention `--depth` and `--reverse` (or whatever the
  actual flags are — confirm against `cmd_graph.go` before writing).
- This task does not touch `AGENTS.md.tmpl` / `CLAUDE.md.tmpl`
  ("not yet built" list there) — those need a separate update once
  this lands. Consider folding that doc edit into the same PR.
