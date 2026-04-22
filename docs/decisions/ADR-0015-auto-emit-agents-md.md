---
title: Auto-emit AGENTS.md on init
status: accepted
coverage: required
date: 2026-04-22
supersedes: []
related: [ADR-0011, ADR-0013]
tags: [agent-interface, standards, init]
---

# Auto-emit AGENTS.md on init

## Context

AGENTS.md has become the de-facto standard in 2025–26 for "agent instructions at repo root" — adopted by Copilot, Codex, Cursor, Jules/Gemini, Factory, Amp, Windsurf, Zed, RooCode, and (via Linux Foundation / Agentic AI Foundation) formalized as a cross-vendor convention. Any tool that expects agents to find it must either write into AGENTS.md or be referenced from AGENTS.md.

## Decision

`docops init` creates (or updates) `AGENTS.md` at the repository root. The file includes a DocOps section that:

1. Names the three folders (`docs/context/`, `docs/decisions/`, `docs/tasks/`).
2. Lists the common CLI commands agents should know (`docops state`, `docops audit`, `docops next`, `docops new task`, `docops get <id>`, `docops graph <id>`).
3. States the alignment invariant (tasks must cite ≥1 ADR or CTX).
4. Points to `docs/STATE.md` as the current-state snapshot.
5. Points to `docs/.index.json` as the computed graph.

If an AGENTS.md already exists, `docops init` inserts or updates a delimited block (`<!-- docops:start -->` / `<!-- docops:end -->`) without touching other content. Subsequent `docops init` runs or an explicit `docops refresh-agents-md` command keep the block current.

## Rationale

- Ride the convergence standard instead of inventing a parallel one.
- Every supported agent tool already reads AGENTS.md; DocOps gets discovery for free.
- A delimited block means DocOps plays well with projects that have existing AGENTS.md content from other tools.

## Consequences

- Updates to the DocOps CLI surface that change common commands also require updating the AGENTS.md template.
- The AGENTS.md block is a kind of documentation; it should stay short and point to the CLI help for detail rather than duplicate it.
- If AGENTS.md as a standard changes materially (unlikely but possible given its newness), DocOps adapts; this ADR may be revised.
- The delimited block format (comments) must be valid markdown and not break any AGENTS.md parser. HTML comments are the safe choice.
