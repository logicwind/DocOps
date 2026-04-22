---
id: "0015"
title: L2 State is pluggable — unified interface pattern
date: 2026-04-16
status: accepted
implementation: done
supersedes: []
superseded_by: null
related: ["0009", "0010", "0011", "0012", "0016"]
tags: [state, architecture, interface, l2]
---

# 0015 · L2 State is pluggable — unified interface pattern

Date: 2026-04-16
Status: accepted
Synthesizes: ADR 0009 (sessions), ADR 0010 (memory), ADR 0011 (artifacts), ADR 0012 (secrets)

## Context

The four L2 State stores — Secrets, Memory, Artifacts, Sessions — have
evolved with different degrees of abstraction:

| Store | Pluggable today? |
|-------|------------------|
| Secrets | Yes (via sops backend) |
| Memory | No (MemoryDb uses bun:sqlite directly) |
| Artifacts | Yes (`ArtifactStore` interface exists) |
| Sessions | Partial (tied to runtime, via ADR 0003) |

This inconsistency is a problem for the layer model (ADR 0001) and for the
OSS/commercial split: hosted deployments need to swap local stores for
cloud-backed ones without touching callers, and that only works if every
store has a clean interface.

## Decision

Every L2 store lives behind an explicit interface. Each interface has a
local default implementation that ships in the OSS core. Alternative
implementations (S3, managed Postgres, cloud KMS, multi-tenant stores) are
additive — they conform to the interface, they don't replace callers.

```
L2 State interfaces                Default impls (OSS, v0.3)
─────────────────────              ──────────────────────────
SecretStore                        AgeSopsSecretStore
MemoryStore                        SqliteMemoryStore
ArtifactStore                      LocalArtifactStore
SessionStore (owned by Runtime)    PiJsonlSessionStore
```

**Interface-level guarantees:**

- Callers in L3 and L4 depend on the interface, never on a concrete impl.
- Implementations are selected via daemon config (`~/.zund/config.yaml`),
  not hardcoded imports.
- Swapping an implementation requires no change to L3/L4 code.

**Adoption status (2026-04-18):**

| Store          | Interface                              | Default plugin                                       |
| -------------- | -------------------------------------- | ---------------------------------------------------- |
| `ArtifactStore` | `@zund/core/contracts/artifacts`      | `@zund/plugin-artifacts-local`                       |
| `MemoryStore`   | `@zund/core/contracts/memory`          | `@zund/plugin-memory-sqlite`                         |
| `SecretStore`   | `@zund/core/contracts/secrets`         | `@zund/plugin-secrets-age-sops`                      |
| `SessionStore`  | `@zund/core/contracts/sessions`        | `@zund/plugin-runtime-pi/sessions/plugin` (bundled with runtime — format is Pi-specific) |

All four are registered through `PluginRegistry` and resolved via
`registry.service<T>(kind)`. Daemon callers obtain store instances through
typed helpers (`memory(state)`, `artifacts(state)`, `secrets(state)`,
`sessions(state)`) rather than importing concrete classes.

Residual concrete-plugin leaks tracked under other ADRs — see ADR 0012 for
the secrets-route utilities (`resolveFleetSecrets`, `readAllSecrets`,
`consumers`) that still import from `@zund/plugin-secrets-age-sops`.

## Consequences

**Makes easier:**

- Commercial / hosted story: swap local stores for multi-tenant cloud
  stores by config.
- Testing: in-memory mock stores for unit tests.
- Clear abstraction boundary for each state concept.
- Federation path (cross-node memory, shared artifacts) is well-defined.

**Makes harder:**

- Requires refactoring existing direct usages of `MemoryDb`. Manageable.
- Adds a layer of indirection. Cost is small because the interfaces are
  narrow (5–10 methods each).
- Interface design has to survive the first alternative implementation.
  Risk of premature abstraction — mitigated by having real cloud-store
  requirements already in view.

## Implementation notes

- Interfaces live in `packages/core/src/contracts/<store>.ts` (moved from
  the daemon during the ADR 0020 plugin extraction).
- Defaults ship as workspace plugins under `packages/plugins/*`. The
  sessions default is bundled with the Pi runtime because the JSONL
  format is runtime-specific; other stores live in their own plugin
  packages.
- Daemon binds concrete plugins through `~/.zund/plugins.yaml`; the
  default manifest registers every bundled impl.
- Realized order: Memory (ADR 0016) → Artifacts + Secrets (ADR 0020
  extraction) → Sessions (formalized 2026-04-18).
