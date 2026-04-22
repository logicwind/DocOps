---
id: "0001"
title: Four-layer architecture (Substrate / State / Orchestration / Access)
date: 2026-04-16
status: accepted
implementation: done
supersedes: []
superseded_by: null
related: ["0002", "0003", "0015"]
tags: [architecture, layering, oss-boundary]
---

# 0001 · Four-layer architecture (Substrate / State / Orchestration / Access)

Date: 2026-04-16
Status: accepted
Supersedes: `docs/archive/zund-plan-v0.3.md` §4 "Architecture Layers" (Library / Fleet / Runtime / Observation)

## Context

The original layering in `archive/zund-plan-v0.3.md` (Library → Fleet → Runtime → Observation)
described a *deployment flow* — YAML config being interpreted by a runtime and
observed via dashboards. That framing made sense for slice 1 but breaks down
as Zund evolves past v0.3:

- It puts "Runtime" (the API + conversations + sessions) as one layer, but the
  substrate (Incus), the state (memory/artifacts/secrets), the orchestration
  (future dispatcher/triggers), and the access surface (CLI/console/API) all
  have very different change rates and concerns.
- It has no place for the planned AI-first task queue + dispatcher work.
- "Observation" as a peer layer overstates its role — observability is a
  cross-cutting concern, not a tier.
- It does not align with the OSS / commercial split (open-source core,
  commercial console/cloud).

## Decision

Adopt a four-layer model based on responsibility and rate of change:

```
L4  ACCESS         CLI · Console · REST · SSE              changes with UX
L3  ORCHESTRATION  Dispatcher · Triggers · Runtime reg     changes with product
L2  STATE          Secrets · Memory · Artifacts · Sessions changes with schemas
L1  SUBSTRATE      Incus · Containers · Fleet reconciler   stable
```

The Agent Runtime interface sits between L1 and L3 — it lets L1 host any
runtime (Pi, VM, SSH) and L3 treat them uniformly. See ADR 0003.

Each layer has one reason to change. The practical test: swapping one
component in a layer should not force changes in other layers.

| Swap | Should only change |
|------|---------------------|
| Incus → Firecracker | L1 |
| SQLite → Postgres | L2 |
| Local dispatcher → Cloud dispatcher | L3 |
| Add mobile app | L4 |

## Consequences

**Makes easier:**

- Clear refactor targets. The current `fleet/executor.ts` mixes L1 (reconcile)
  and L3 (launch agents); the new model names the split.
- OSS / commercial boundary aligns with layer boundary. L1 and single-tenant
  L2 / local L3 are always OSS; hosted multi-tenant L2 and L3 marketplace
  features are commercial.
- New work (task queue, dispatcher, triggers) has a clear home in L3 without
  polluting the substrate.
- Runtime abstraction (ADR 0003) has a natural seam between L1 and L3.

**Makes harder:**

- More layers to explain to newcomers than the original three-layer intuition.
- Cross-cutting concerns (observability, configuration, auth) don't fit as
  layers — they sit across all four. Must be called out explicitly rather
  than hidden in one tier.

**Renames from the old model:**

- Old "Library" (skills/tools YAML) — absorbed into L1 fleet parsing.
- Old "Fleet Definition" — absorbed into L1 fleet parsing.
- Old "Runtime (API)" — split across L2 state, L3 orchestration, L4 access.
- Old "Observation" — recast as a cross-cutting concern, not a layer.
