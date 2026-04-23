---
title: Restore /docops:* skills for read commands + add orchestration skills
status: backlog
priority: p1
assignee: unassigned
requires: [ADR-0013, ADR-0019, ADR-0026]
depends_on: []
---

# Restore /docops:* skills for read commands + add orchestration skills

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
  (Partially addressed in commit 5d613ee — template verbosity trim
  also removed the stale "not yet built" block. Still worth revisiting
  once the five read-skills land.)

## Phase 2 — orchestration skills (adds ADR-0026 citation)

The read-command skills above are thin CLI wrappers. Separately ship a
small pack of *orchestration* skills that wrap multi-step DocOps
workflows — the pattern gstack/gsd use successfully (frontmatter points
at a workflow document; the skill is a slim shell that delegates).

Orchestration skills to ship:

- `/docops:progress` — read `docs/STATE.md` + `docops audit` + `docops
  next`, summarize situation, recommend the next action. Equivalent of
  `gsd-progress`.
- `/docops:do <freeform>` — NL intent router. Map "start a new decision"
  → `/docops:new-adr`, "what's next" → `/docops:progress`, etc.
  Equivalent of `gsd-do`.
- `/docops:plan` — given a CTX filename or ID, draft an ADR + one or
  more tasks that cite it. Human-confirmed before write. Uses
  `docops new adr --body -` and `docops new task --body -` under the
  hood to avoid the stub-then-rewrite round-trip.
- `/docops:close` — given a task ID, update its status to `done`, run
  `docops refresh`, and stage a commit with a templated message. Agent
  fills in the "what changed" sentence.
- `/docops:review` — wraps `docops review` (see ADR-0026 / TP-029). For
  each stale-review ADR: print ADR body + relevant commits, prompt the
  agent to assess drift, offer `--mark` on confirmation.

Acceptance for Phase 2:

- Five new skill files under `templates/skills/docops/` with the same
  frontmatter shape as existing skills (`name`, `description`, body).
- Skills are thin: bodies should read like "run `docops X`, interpret
  output, prompt user on ambiguity." No embedded domain knowledge that
  belongs in CLI help text.
- `templates/skills_lint_test.go` extended to cover the orchestration
  skills' frontmatter.
- `docops upgrade` on a v0.2.x-scaffolded repo cleanly adds the five
  new skill files (`+ new` lines only).
- `/docops:review` cannot ship until TP-029 lands. Track that edge as
  an ordering constraint, not a blocker for the rest of Phase 2.

## Phase ordering

Phase 1 (read-command wrappers) unblocks agents immediately and is
self-contained. Phase 2 depends on TP-029 for `/docops:review` but the
other four orchestration skills can ship in parallel with or ahead of
TP-029. Split into two PRs if it helps keep the diff reviewable.
