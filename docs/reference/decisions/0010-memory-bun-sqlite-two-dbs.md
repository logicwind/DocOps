---
id: "0010"
title: "Memory — bun:sqlite, two DBs, no Mastra"
date: 2026-04-16
status: accepted
implementation: done
supersedes: []
superseded_by: null
related: ["0015", "0016"]
tags: [state, memory, sqlite, l2]
---

# 0010 · Memory — bun:sqlite, two DBs, no Mastra

Date: 2026-04-16
Status: accepted
Related: ADR 0015 (L2 pluggability), ADR 0016 (memory pluggability direction)

## Context

Agents need two distinct forms of memory:

1. **Session history** — the JSONL conversation log, one file per session,
   written by Pi. Ephemeral (GC'd after retention window). Indexed by the
   daemon for fast lookup without parsing every JSONL file.
2. **Facts and working memory** — long-lived, queryable knowledge. Facts
   need full-text search. Working memory is a per-agent scratchpad.

Initial plan considered Mastra's `@mastra/memory` + `@mastra/libsql`
(validated in POC 05 and POC 09). It works but brings dependencies, a
particular schema opinion, and an API that doesn't match Zund's
fleet-scoped concepts (agent, team, fleet scopes).

## Decision

Drop Mastra. Use `bun:sqlite` directly with two SQLite databases:

- **`~/.zund/data/sessions.db`** — ephemeral index of Pi JSONL files. Rows
  point at session file paths; GC'd per retention policy.
- **`~/.zund/data/memory.db`** — permanent store. Tables:
  - `facts` (id, agent, content, scope, created_at, embedding BLOB)
  - `facts_fts` (FTS5 virtual table for full-text search)
  - `working_memory` (agent, scope, content, updated_at)

The `MemoryDb` class in `packages/daemon/src/memory/db.ts` wraps both
databases and exposes: `saveFact`, `searchFacts`, `listFacts`,
`getWorkingMemory`, `setWorkingMemory`.

Scopes are agent-controlled: an agent decides whether a fact is agent-scoped,
team-scoped, or fleet-scoped on save.

Embeddings are optional and async, never block response — see
`packages/daemon/src/memory/embeddings.ts`. Provider is configurable via
`defaults.agent.memory.embeddings.provider`: `none | ollama | openai`.

## Consequences

**Makes easier:**

- Zero memory-related dependencies (`bun:sqlite` ships with Bun).
- Schema matches Zund's concepts directly — no translation layer.
- FTS5 gives keyword search without extra infra.
- Predictable failure modes — it's just SQLite.

**Makes harder:**

- No built-in per-agent storage isolation. All agents write to the same
  `memory.db`. This is fine for v0.3 single-node but becomes a question for
  federation / commercial multi-tenant. See ADR 0016 for the direction.
- `MemoryDb` is used directly by callers; not yet behind an interface.
  Pluggability (Postgres, hosted memory, per-agent SQLite) requires the
  `MemoryStore` abstraction from ADR 0016.
- Vector search is BLOB-based and slow above ~10k facts. Acceptable for
  v0.3; noted as a future limit.

## Implementation notes

- Lives at `packages/daemon/src/memory/`.
- Pi tools `memory_save`, `memory_search`, `working_memory_update` are
  registered in the Pi extension (ADR 0005) and call through to `MemoryDb`.
- Embedding queue runs on a 2-second batch timer; no-op when provider is
  `none`.
