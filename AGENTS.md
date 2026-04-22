# AGENTS.md — working on the DocOps source

You are inside the **DocOps source repository**. DocOps is the product being
built here. This file orients agents (Claude Code, Cursor, Aider, Codex,
Copilot, Windsurf, Zed, etc.) to the codebase.

If you were looking for the user-facing "how to use DocOps in my project"
guidance, see `templates/AGENTS.md.tmpl`. That is the file `docops init`
will eventually emit into user repositories. It is not this file.

## What DocOps is

A typed project-state substrate for LLM-first software development. Three doc
types (Context, Decision, Task) with YAML frontmatter, a computed index, a
CLI, and a coverage-audit workflow. See `docs/context/CTX-001-docops-vision.md`
for the full vision and `docs/context/CTX-004-user-constraints.md` for
non-negotiable guardrails.

## Meta vs. product — do not conflate

This repo has two layers. Files in one never leak into the other:

| Layer | Purpose | Location |
|---|---|---|
| **Meta** | Governs this repo's own development. We dog-food DocOps concepts manually until the CLI exists. | `docs/context/`, `docs/decisions/`, `docs/tasks/`, `docops.yaml`, `docs/STATE.md`, this file |
| **Product** | What DocOps emits into user repos when they run `docops init`. | `templates/` (today), `src/`, `bin/` (once CLI scaffolding lands) |

Why this matters: an agent might be tempted to "fix" root `AGENTS.md` by
pasting in user-facing command explanations. Don't. That content belongs in
`templates/AGENTS.md.tmpl`. See `docs/decisions/ADR-0016-meta-vs-product-separation.md`.

## Bootstrap state (important)

The CLI does not yet exist. Commands like `docops validate`, `docops index`,
`docops audit` are being built under TP-001..TP-010. Until those tasks ship:

- Treat the invariants defined in ADR-0002 (schema) and ADR-0004 (task alignment) as **convention**, not enforcement.
- Maintain `docs/STATE.md` by hand. It is marked as sample content; TP-005 will generate it.
- Do not invent CLI commands that do not exist yet. If a workflow needs a command that has not been built, propose a task first.

## Working on this repo

1. Read `docs/STATE.md` for where we are.
2. Read `docs/tasks/` for queued work. Check `depends_on` before picking one up.
3. When adding or modifying a doc in `docs/`, hand-follow the frontmatter spec in `docs/decisions/ADR-0002-bare-minimum-frontmatter.md`. The files are the data until the CLI validates them.
4. If you create a new task, ensure its `requires:` cites ≥1 existing ADR or CTX (ADR-0004 alignment rule).
5. If your work introduces a new decision worth recording, write an ADR before coding.

## Structure quick reference

```
AGENTS.md                      ← you are here (meta: working on DocOps)
docops.yaml                    ← meta: this repo's DocOps config
docs/
  STATE.md                     ← meta: hand-maintained snapshot
  context/CTX-*.md             ← meta: stakeholder/author inputs
  decisions/ADR-*.md           ← meta: design decisions
  tasks/TP-*.md                ← meta: phase-1 backlog
templates/
  AGENTS.md.tmpl               ← product: emitted to user repos
  docops.yaml.tmpl             ← product: emitted to user repos
src/                           ← product (forthcoming, TP-001)
bin/                           ← product (forthcoming, TP-001)
```

## Pairs well with

- **Native plan-mode** in your IDE — DocOps explicitly does not replace it.
- **GStack** role skills if you have them installed — useful perspective, independent of DocOps scope.
