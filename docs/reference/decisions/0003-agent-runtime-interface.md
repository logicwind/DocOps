---
id: "0003"
title: Agent runtime interface
date: 2026-04-16
status: accepted
implementation: done
supersedes: []
superseded_by: null
related: ["0002", "0005", "0009", "0013", "0020"]
tags: [architecture, runtime, refactor]
---

# 0003 · Agent runtime interface

Date: 2026-04-16
Status: accepted
Related: ADR 0002 (stream protocol), ADR 0005 (Pi as initial runtime), ADR 0009 (session storage), ADR 0020 (plugin architecture)

## Context

Today the Pi runtime is hardcoded throughout the daemon:

- `fleet/executor.ts` (663 lines) launches agents via `pi/launcher.ts` directly.
- `AgentHandle` exposes `rpcSession: AgentRpcSession` — Pi-specific type.
- Container naming is fixed (`zund-${agent.name}`) even though container naming
  is a runtime concern.
- Session storage (host-mounted JSONL) is Pi-specific but lives in the generic
  `sessions/` dir.
- `tools:` field in agent YAML lists Pi built-in tools; another runtime would
  have a different vocabulary.

An investigation on the Pi isolation boundary found the Pi runtime is
~80% isolated (clean `pi/` directory, runtime-agnostic fleet types) but the
remaining 20% leaks into executor, API response types, and the wire format.

Without a defined Runtime interface, any new runtime (VM-based, SSH-based,
alternative LLM harness) requires parallel code paths and client-side
branching.

## Decision

Extract a `Runtime` interface that every agent runtime implementation
conforms to. L1 (substrate) and L3 (orchestration) talk to the interface,
not to Pi directly.

The interface and its supporting types live in `@zund/core/contracts/runtime.ts`
so that both the daemon and future plugin implementations can depend on them
without circular imports. No daemon internals in contract signatures.

```typescript
interface Runtime {
  readonly name: string;

  launch(agent: AgentResourceRef, ctx: RuntimeContext): Promise<RuntimeSession>;
  session(agentName: string): RuntimeSession | null;
  stop(agentName: string): Promise<void>;

  // Mount service bridges — wires L2 contracts into runtime-native mechanisms
  mountBridges(session: RuntimeSession, services: ServiceContracts): void;

  // Native runtime events — translated to zund://stream/v1 by L3
  events(): AsyncIterable<RuntimeEvent>;
}

interface RuntimeSession {
  readonly agentName: string;
  message(payload: MessagePayload): AsyncIterable<RuntimeEvent>;
  close(): Promise<void>;
}
```

Key additions from the original proposal:

- **`mountBridges(session, services)`** — ADR 0020's bridge pattern. Runtimes
  ship per-service translators (e.g. Pi's `memory-bridge.ts` → Pi tool registration
  → `MemoryStore` contract). The daemon calls `mountBridges` after launch, once
  services are bound.
- **`ServiceContracts`** — a bag of currently-bound L2 service instances.
  Optional fields; runtimes mount only bridges for services that are present.
- **`AgentResourceRef`** — minimal agent reference passed to `launch()`, not
  the full `AgentResource` with fleet YAML details. The runtime doesn't need
  secrets/role refs.
- **`RuntimeContext`** — value object with container name, env vars, mounts,
  skills, provider config, fleet metadata. No daemon-internal types.

Code moves:

```
packages/core/                              ← NEW — contract interfaces
  src/contracts/runtime.ts                    ← Runtime, RuntimeSession, RuntimeContext, etc.
  src/index.ts                               ← barrel export

packages/daemon/src/pi/                     ← MOVED
  launcher.ts → agents/runtimes/pi/launcher.ts
  rpc.ts      → agents/runtimes/pi/rpc.ts
  config.ts   → agents/runtimes/pi/config.ts
  extension.ts → agents/runtimes/pi/extension.ts
  extension-writer.ts → agents/runtimes/pi/extension-writer.ts

packages/daemon/src/agents/registry.ts       ← NEW — name → Runtime resolution
```

A `packages/core/` workspace package is created to hold all contract
interfaces. The daemon depends on `@zund/core` via `workspace:*`. When
ADR 0020 Phase 1 lands, `contracts/` → `@zund/core/contracts/` is a direct
file move — no interface redesign.

`AgentResource` gains a `runtime: string` field (default `"pi"`) that the
registry uses to dispatch.

The `tools:` field becomes runtime-specific vocabulary. Document the Pi
vocabulary in the Pi runtime module; other runtimes declare their own.
Consider renaming to `capabilities:` at the YAML level with a neutral
semantic (ADR 0013 tracks the current Pi-specific use; revisit when a
second runtime appears).

## Consequences

**Makes easier:**

- Multi-runtime support. A VM runtime (QEMU-backed) or SSH runtime (remote
  host) drops in behind the interface.
- Dispatcher (L3) selects agent *and* runtime when routing a task.
- Clean seam for testing — mock runtime for unit tests without Incus.
- Stream protocol (ADR 0002) translator becomes a map keyed by runtime name.

**Makes harder:**

- Requires refactoring `fleet/executor.ts` to route through the registry.
  Estimated 1 day of careful restructuring.
- `AgentHandle` public shape changes — `rpcSession` becomes
  `session: RuntimeSession`. Downstream types in CLI/console transport must
  update (currently duplicated — see ADR on @zund/types package, TBD).
- Session access becomes per-runtime; the daemon no longer sees a single
  session shape.

## Designs for ADR 0020

The `Runtime` interface and supporting types are structured for extraction
to `@zund/core/contracts/runtime.ts` (they already live there). The registry
(`agents/registry.ts`) follows the same `kind → name → instance` lookup
pattern that `core/registry.ts` will use. When ADR 0020 Phase 1 adds the
full plugin registry, this becomes `core.registry.runtime("pi")` — no lookup
semantics change.

The `mountBridges` method is the ADR 0020 bridge pattern expressed at the
contract level. When 0020 Phase 2 extracts runtimes to
`packages/plugins/runtime-pi/`, the bridges move alongside as
`bridges/memory-bridge.ts`, `bridges/artifacts-bridge.ts`, etc.

The pre-existing ~42 `tsc --noEmit` errors in `rpc.ts`, `differ.ts`,
`incus/client.ts`, and API body types are unrelated to this ADR and need
a dedicated cleanup pass.
