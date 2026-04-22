---
title: Meta vs. product layer separation during dog-food
status: accepted
coverage: required
date: 2026-04-22
supersedes: []
related: [ADR-0014, ADR-0015]
tags: [repo-layout, dog-food, philosophy]
---

# Meta vs. product layer separation during dog-food

## Context

This repository is the DocOps source code AND a DocOps-managed project (dog-food). Both layers want to live in the same repo but must not be conflated. Without an explicit convention:

- `AGENTS.md` at root confuses agents: is this repo using DocOps or building it?
- Commands referenced in docs may not exist yet (the CLI is being built).
- Template content that `docops init` eventually emits gets mistaken for configuration that governs this repo's own development.

## Decision

The repo has two explicit layers with non-overlapping file locations:

**Meta layer** — governs this repo's own development, here and now:
- Root `AGENTS.md` — agent orientation for working on the DocOps source.
- `docs/context/`, `docs/decisions/`, `docs/tasks/` — project management of DocOps itself, using the DocOps conventions manually until the CLI exists.
- `docops.yaml` at root — this repo's own DocOps config (dog-food).
- `docs/STATE.md` — hand-maintained until TP-005 ships.

**Product layer** — what DocOps emits into user repos when they run `docops init`:
- `templates/AGENTS.md.tmpl`
- `templates/docops.yaml.tmpl`
- `templates/skills/` (forthcoming)
- Eventually `src/`, `bin/` — the CLI source and compiled binary.

Files in the two layers serve different audiences. Editing one to fix the other is a category error.

## Rationale

- Eliminates the paradox that shows up when AGENTS.md tells agents to run a binary that does not yet exist.
- Makes the dog-food genuine: our own `docs/` really is managed with DocOps conventions, validated by future tooling.
- Gives the product artifacts a durable home (`templates/`) that predates the CLI implementation.

## Consequences

- Root `AGENTS.md` is rewritten to be explicit about this being the DocOps source repo, with a note that bootstrap conventions are manual until the CLI lands.
- `templates/AGENTS.md.tmpl` holds the "user-facing" content that `docops init` will eventually copy into user repos.
- The `docops init` implementation (TP-007) reads from `templates/` rather than carrying template content inside its own code.
- Agents and humans must not cross the layer boundary — do not paste user-facing guidance into root AGENTS.md, and do not paste meta-level guidance into templates.
- When behaviour changes (e.g., a new CLI command), both layers may need updates; the product template is the canonical source and root AGENTS.md mirrors only what is relevant to working on the source.
