---
title: Four-concept model — CTX, ADR, Task, Index
status: accepted
coverage: required
date: 2026-04-22
supersedes: []
related: []
tags: [schema, architecture, core]
---

# Four-concept model — CTX, ADR, Task, Index

## Context

DocOps needs a minimal vocabulary that maps cleanly to how software gets built with LLMs. Existing tools fall into two traps:
- Blurring task / decision / requirement boundaries (Backlog.md, Markplane).
- Requiring many doc types that are rarely used (formal SDD frameworks).

## Decision

DocOps uses three source doc types plus one computed artifact:

1. **Context (CTX)** — stakeholder inputs. PRDs, memos, research, interviews, Slack-thread pastes. Heterogeneous shapes, typed via a `type:` field configurable per project. Lives in `docs/context/`.

2. **Decisions (ADR)** — architecture and process decisions. Based on the Nygard/MADR tradition with lifecycle states. Lives in `docs/decisions/`.

3. **Tasks (TP)** — units of work the team and agents execute. Must cite ≥1 ADR or CTX. Lives in `docs/tasks/`.

4. **Index** — computed artifact (`docs/.index.json`) containing the union of source frontmatter + derived fields (reverse edges, resolved references, staleness, summaries). Never hand-edited. Agents query it; the CLI emits it.

## Rationale

- CTX separates the *why* from the *how*. Without it, stakeholder intent collapses into ticket bodies and is lost.
- ADRs separate the *how we chose* from the *what we do*. Without them, decisions are re-litigated per task.
- Tasks separate *work* from *justification*. The citation rule (see ADR-0004) forces the link.
- Index separates what humans write (minimal) from what agents read (enriched).

## Consequences

- Any project adopting DocOps must scaffold these three folders. A repo without them cannot be DocOps-valid.
- Tools, CLI commands, and skill names mirror the three types: `docops new ctx`, `docops new adr`, `docops new task`.
- New doc types cannot be added casually. If a project wants a fourth type, they push it into CTX with a new `type:` value rather than inventing a new folder.
