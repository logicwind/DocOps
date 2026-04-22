---
id: "0016"
title: Memory pluggability direction (MemoryStore interface)
date: 2026-04-16
status: accepted
implementation: done
supersedes: []
superseded_by: null
related: ["0010", "0015"]
tags: [state, memory, interface, l2]
---

# 0016 · Memory pluggability direction (MemoryStore interface)

Date: 2026-04-16
Status: accepted
Implementation: done (shipped on `feat/plugin-architecture` Phase 2)
Related: ADR 0010 (current memory impl), ADR 0015 (L2 pluggability)

## Context

ADR 0010 captures the current memory implementation: `bun:sqlite`, two
DBs, direct usage of the `MemoryDb` class. ADR 0015 commits Zund to
having every L2 store behind an interface.

Memory is the one L2 store that does not yet have an interface. Two
open questions shape the design:

1. **Per-agent vs shared storage.** Today all agents write to the same
   `memory.db`. Should the v2 design give each agent its own database?
   Carried forward from `archive/pending-clarity.md` item G.
2. **Single-node vs federated.** The commercial / hosted Zund needs
   multi-tenant memory, cross-node consistency, and auth scoping. The
   interface shape must accommodate this without leaking tenancy into
   local single-node usage.

## Decision

Define a `MemoryStore` interface with the method set `MemoryDb` already
exposes:

```typescript
interface MemoryStore {
  saveFact(args: SaveFactArgs): Promise<Fact>;
  searchFacts(args: SearchFactsArgs): Promise<Fact[]>;
  listFacts(args: ListFactsArgs): Promise<Fact[]>;
  deleteFact(id: string): Promise<void>;

  getWorkingMemory(agent: string, scope: string): Promise<WorkingMemory | null>;
  setWorkingMemory(agent: string, scope: string, content: string): Promise<void>;
  clearWorkingMemory(agent: string, scope: string): Promise<void>;
}
```

**Defer but acknowledge:** the per-agent vs shared question. The interface
design doesn't force a choice — a per-agent implementation would
instantiate one store per agent; a shared one uses the existing single
database keyed by agent. That choice is made in the configuration /
factory layer, not in the interface.

**First impl:** `SqliteMemoryStore` in `@zund/plugin-memory-sqlite` wraps
the existing `MemoryDb` with the interface. The daemon resolves it through
`state.registry.service<MemoryStore>("memory")`; no daemon code imports
`MemoryDb` directly.

**Second impl (future):** `HostedMemoryStore` for commercial deployments
(cloud-backed, multi-tenant). Not in scope now; the interface is designed
to accommodate it without future renames.

**Embedding provider** stays a per-impl concern. The v0.3 `SqliteMemoryStore`
ships with FTS5 and no vector index, so there is no embedding dependency to
inject. When a future impl needs embeddings, it will accept the provider as
a constructor argument (DI), not as part of `MemoryStore`. The interface
stays free of retrieval-strategy details.

## Consequences

**Makes easier:**

- Memory joins the other pluggable L2 stores under ADR 0015.
- Per-agent storage becomes an implementation choice, reversible without
  changing callers.
- Hosted / cloud memory path unblocked.
- Callers (Pi extension tools, memory routes) stop depending on concrete
  `MemoryDb`.

**Makes harder:**

- One more interface to design and maintain.
- The per-agent vs shared decision is deferred but not eliminated — will
  need resolution when hosted implementation is built.
- Migration: existing callers of `MemoryDb` must switch to `MemoryStore`.
  Mechanical refactor, one PR.

## Resolved open questions (2026-04-18)

- **Working memory in the same interface: yes.** `getWorkingMemory`,
  `setWorkingMemory`, `patchWorkingMemory`, `listWorkingMemoryScopes`,
  `deleteWorkingMemory`, and `clearWorkingMemoryByPrefix` live on
  `MemoryStore`. Facts and working memory share a lifecycle (same
  agent/scope keyspace, opened and closed together) and every known caller
  wants one handle.
- **Embedding provider: DI, not interface.** See Decision. The v0.3 SQLite
  impl uses FTS5 and takes no embedding dependency; the interface stays
  free of retrieval-strategy details.
- **Scope typing: string-by-convention, not enum.** The sqlite impl stores
  scope as TEXT and callers compose `agent:{name}`, `team:{name}`,
  `fleet:{name}`, and `thread:{id}`. A closed TypeScript enum would prevent
  future scope families (session, task, etc.) and forces every plugin to
  stay in lockstep with core. Scope grammar is documented on the contract;
  enforcement stays in callers.

## Remaining forward question

- **Per-agent vs shared storage for the hosted impl.** Still deferred —
  the current local impl is shared, and nothing about the interface forces
  the choice. Will be resolved when `HostedMemoryStore` lands.
