---
id: "0020"
title: "Plugin architecture — two-tier with runtime + service plugins and bridge pattern"
date: 2026-04-16
status: accepted
implementation: partial
supersedes: []
superseded_by: null
related: ["0003", "0009", "0010", "0011", "0012", "0015", "0016", "0017", "0018", "0019"]
tags: [plugins, architecture, runtime, bridges, l1, l2, l3]
---

# 0020 · Plugin architecture — two-tier with runtime + service plugins and bridge pattern

Date: 2026-04-16
Status: draft
Related: ADR 0003 (runtime interface), ADR 0009-0012 (L2 stores), ADR 0015-0016
(L2 pluggability), ADR 0017 (humans), ADR 0018 (lifecycle), ADR 0019 (MCP)

## Context

Zund's four-layer architecture (ADR 0001) anticipates pluggability at every
layer. Several ADRs already hint at it:

- ADR 0003: Runtime is an interface that multiple implementations satisfy.
- ADR 0015: L2 State is pluggable (direction).
- ADR 0016: MemoryStore pluggability is pending.
- ADR 0019: MCP support requires a sidecar lifecycle system.
- ADR 0017: Humans-as-fleet-members imply messaging gateways, which are
  inherently pluggable (Telegram, Slack, email, WhatsApp).

Without a unified plugin architecture, each of these is planned or built
in isolation. The result is a daemon that grows ad-hoc extension points
with no shared manifest, lifecycle, or isolation story. This blocks:

- **Multi-runtime support** (ADR 0018 Phase 2) — adopting Hermes or
  OpenClaw as runtimes requires a clean seam the daemon doesn't have.
- **OSS ↔ Pro seam** — commercial-tier features (federated memory,
  hosted secrets, enterprise auth) need a principled boundary between
  "bundled with OSS" and "contributed / licensed."
- **Enterprise deployment** — swapping SQLite for Postgres, local CAS
  for S3, in-process bus for NATS, etc. is currently a code fork, not a
  configuration change.

A further complication: **runtimes are not like other plugins.** A
runtime owns the agent's execution model (process, event loop, tool
invocation). Every other plugin provides a *service* (memory, secrets,
artifacts) the agent consumes. If runtimes are treated as peers to
services, either service plugins end up with runtime-specific branches
(N×M sprawl) or we ship per-runtime variants of every service plugin.

## Decision

Zund adopts a **two-tier plugin architecture**:

1. **Runtime tier** — runtimes are special. Each runtime plugin
   implements the `Runtime` interface and bundles **bridge adapters**
   that translate core service contracts into the runtime's native
   mechanisms.
2. **Service tier** — pure, runtime-agnostic plugins implementing
   semantic contracts (MemoryStore, SecretStore, ArtifactStore, Bus,
   Queue, Dispatcher, MCPHost, Gateway, Auth, Observer, etc.).

A core package defines interfaces, manifest schema, registry, and
lifecycle. The daemon consumes plugins only through the registry —
never by direct module import.

### Tier structure

```
┌─────────────────────────────────────────────────────────────┐
│ Runtime tier (special)                                       │
│   Each plugin: implements Runtime + ships bridges/           │
│     runtime-pi        (Pi agent core)                        │
│     runtime-hermes    (adopted per ADR 0018 Phase 2)         │
│     runtime-openclaw  (adopted per ADR 0018 Phase 2)         │
│     runtime-vm        (future: QEMU-backed)                  │
│     runtime-ssh       (future: remote host)                  │
└─────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────┐
│ Service tier (pure, runtime-agnostic)                        │
│   kind: memory         memory-sqlite (default)               │
│                        memory-postgres, memory-pinecone      │
│   kind: secrets        secrets-age-sops (default)            │
│                        secrets-vault, secrets-aws-kms        │
│   kind: artifacts      artifacts-local (default)             │
│                        artifacts-s3, artifacts-gcs           │
│   kind: images         images-local (default)                │
│                        images-harbor, images-ecr             │
│   kind: bus            bus-inproc (default)                  │
│                        bus-nats, bus-redis                   │
│   kind: queue          queue-sqlite (default)                │
│                        queue-postgres, queue-nats            │
│   kind: dispatcher     dispatcher-rules (default)            │
│                        dispatcher-llm                        │
│   kind: policy         policy-yaml (default)                 │
│                        policy-opa                            │
│   kind: mcp-host       mcp-sidecars (default, ADR 0019)      │
│                        mcp-host-aggregator (Option C)        │
│   kind: gateway        (no default; opt-in only)             │
│                        gateway-telegram, gateway-slack,      │
│                        gateway-email, gateway-whatsapp       │
│   kind: auth           auth-local (default)                  │
│                        auth-oidc, auth-saml                  │
│   kind: observer       observer-stderr (default)             │
│                        observer-otel, observer-datadog       │
└─────────────────────────────────────────────────────────────┘
```

### Repository layout

```
packages/
  core/                          ← NEW — interfaces + registry + lifecycle
    src/
      plugin.ts                  ← Plugin<Kind> interface
      registry.ts                ← kind → instance resolution
      manifest.ts                ← YAML schema + loader
      context.ts                 ← ZundContext passed to plugins
      contracts/
        runtime.ts               ← Runtime interface
        memory.ts                ← MemoryStore contract
        artifacts.ts             ← ArtifactStore contract
        secrets.ts               ← SecretStore contract
        bus.ts                   ← Bus contract
        queue.ts                 ← Queue contract
        dispatcher.ts            ← Dispatcher contract
        mcp-host.ts              ← MCPHost contract
        gateway.ts               ← Gateway contract
        auth.ts                  ← Auth contract
        observer.ts              ← Observer contract
        policy.ts                ← Policy contract
        images.ts                ← ImageStore contract

  daemon/                        ← EXISTS — orchestrates, owns registry
    src/                         ← imports ONLY from core.registry
      api/                       ← HTTP/SSE router (consumes plugins)
      fleet/                     ← reconciler (consumes plugins)
      no longer imports: pi/, memory/, artifacts/, secrets/ directly

  plugins/                       ← NEW — bundled OSS defaults
    runtime-pi/
      src/
        runtime.ts               ← implements Runtime
        bridges/
          memory-bridge.ts       ← Pi.registerTool("memory_save") → MemoryStore
          artifacts-bridge.ts    ← Pi artifact tool → ArtifactStore
          secrets-bridge.ts      ← env injection at launch
          mcp-bridge.ts          ← Pi tool registration for MCP (ADR 0019)
    memory-sqlite/
    artifacts-local/
    secrets-age-sops/
    images-local/
    bus-inproc/
    queue-sqlite/
    dispatcher-rules/
    policy-yaml/
    mcp-sidecars/
    auth-local/
    observer-stderr/

  plugins-contrib/               ← NEW — optional, not bundled
    runtime-hermes/
    runtime-openclaw/
    memory-postgres/
    artifacts-s3/
    secrets-vault/
    bus-nats/
    gateway-telegram/
    gateway-slack/
    ...
```

### The `Plugin` interface

```typescript
// packages/core/src/plugin.ts
export type PluginKind =
  | "runtime" | "memory" | "secrets" | "artifacts" | "images"
  | "bus" | "queue" | "dispatcher" | "policy" | "mcp-host"
  | "gateway" | "auth" | "observer";

export interface ZundPlugin<K extends PluginKind, Instance> {
  kind: K;
  name: string;                  // "pi", "age-sops", "postgres"
  version: string;
  provides: string[];            // e.g. ["runtime:pi"], reference keys
  requires?: string[];           // e.g. ["bus"] — must be bound before init
  init(config: unknown, ctx: ZundContext): Promise<Instance>;
  shutdown?(): Promise<void>;
  health?(): Promise<"ok" | "degraded" | "fail">;
}
```

Each kind has a typed `Instance` interface living in `core/contracts/*.ts`.

### The runtime tier — bridges

Runtime plugins have a special shape: they implement the `Runtime`
contract **and** ship bridge adapters. A bridge is a small per-runtime
translator that wires a core service contract (MemoryStore,
ArtifactStore, etc.) to the runtime's native mechanism.

```typescript
// packages/plugins/runtime-pi/src/bridges/memory-bridge.ts
import type { MemoryStore } from "@zund/core/contracts/memory";
import type { ExtensionAPI } from "@mariozechner/pi-coding-agent";

export function mountMemoryBridge(pi: ExtensionAPI, memory: MemoryStore) {
  pi.registerTool({
    name: "memory_save",
    label: "Save fact",
    description: "Persist a fact to fleet memory",
    parameters: Type.Object({ fact: Type.String(), tags: Type.Array(Type.String()) }),
    async execute(_id, { fact, tags }) {
      await memory.saveFact({ fact, tags, source: "agent" });
      return { content: [{ type: "text", text: "Saved." }] };
    },
  });
}
```

When Pi launches in a container, the runtime plugin loads bridges
corresponding to every service plugin currently bound in the registry.
The agent sees native Pi tools; the daemon sees contract calls. The
service plugin knows nothing about Pi.

### Runtimes may provide service kinds

Some runtimes have strong native implementations of services (e.g.,
Hermes's native memory, Hermes's native MCP). A runtime plugin may
declare additional provides:

```typescript
// runtime-hermes manifest
{
  kind: "runtime",
  name: "hermes",
  provides: [
    "runtime:hermes",
    "memory:hermes-native",      // opt-in alternative MemoryStore
    "mcp-host:hermes-native",    // opt-in alternative MCPHost
  ],
  version: "0.1.0",
}
```

**Resolution rule: explicit wins.** Fleet YAML chooses which provider
binds each kind. Runtimes only "become" another plugin kind when the
user explicitly binds their name. This keeps defaults predictable and
native-first runtimes opt-in.

### Fleet YAML

```yaml
# fleet/plugins.yaml
kind: Plugins
uses:
  runtime: [runtime-pi, runtime-hermes]   # multiple runtimes allowed
  memory: memory-postgres                  # swap default
  artifacts: artifacts-s3
  secrets: secrets-vault
  bus: bus-nats                            # scale-out
  queue: queue-postgres
  gateway: [gateway-telegram, gateway-slack]
  # dispatcher, policy, mcp-host, auth, observer, images omitted → defaults

config:
  memory-postgres:
    url: { secret: POSTGRES_URL }
  artifacts-s3:
    bucket: zund-artifacts
    region: us-east-1
  gateway-telegram:
    bot-token: { secret: TELEGRAM_BOT_TOKEN }
```

### Lifecycle

```
daemon startup
  1. core.manifest.load(fleet/plugins.yaml)
  2. for each plugin declared:
       import module (bundled or contrib)
       resolve requires (topological order)
       call plugin.init(config, ctx) → instance
       core.registry.register(kind, name, instance)
  3. runtime plugins mount bridges against currently bound services
  4. daemon wires its HTTP routes and reconciler through registry only
  5. on SIGTERM: core.registry.shutdownAll() (reverse topological order)
```

### Registry lookup semantics

```typescript
// Runtime tier — by name
const rt = core.registry.runtime("pi");
await rt.launch(agent, ctx);

// Service tier — by kind (single bound) or name (specific)
const memory = core.registry.service("memory");             // bound default
const explicit = core.registry.service("memory", "sqlite"); // specific

// Optional: list alternatives
const all = core.registry.alternatives("memory"); // returns list
```

## Challenges and open questions

- **Plugin discovery.** Bundled plugins are always available via
  `packages/plugins/*`. Contrib plugins: npm packages installed in the
  daemon workspace? Or local path references in YAML? Probably both.
- **Dependency injection cycles.** Runtime-pi requires bus; bus-nats
  requires observer; observer-otel requires secrets. Topological sort
  at startup; fail cleanly on cycles.
- **Plugin versioning.** Plugins should declare compatible core SDK
  version range. Daemon refuses to load incompatible plugins.
- **Hot reload.** Not in MVP. Shutdown + restart is the supported
  path. Adding hot reload later is additive.
- **Sidecar plugins vs in-process.** Gateway/MCP-host plugins may
  benefit from sidecar isolation; memory/secrets/artifacts are
  low-latency and belong in-process. Plugin manifest declares
  `transport: inproc | sidecar`. MVP: both can be inproc. Sidecar
  support added with the first plugin that needs it.
- **Runtime overrides of service kinds.** When `runtime-hermes` binds
  `memory: hermes-native`, Pi agents in the same fleet can't use it
  (it's Hermes-internal). Policy: runtime-provided services are only
  available to that runtime's agents. Daemon enforces this at binding
  time.
- **Bridges and lifecycle.** Bridges mount inside the agent container
  (runtime-owned) but reference daemon-side service instances. Requires
  a stable runtime↔daemon API (Pi RPC or Unix socket). Pin this in
  runtime plugin implementation, not core.
- **Testing.** Plugins should have contract tests — a MemoryStore
  plugin passes the same test suite regardless of implementation. Ship
  `@zund/core/contracts/testkit` for plugin authors.
- **Configuration schema validation.** Each plugin declares a config
  schema (TypeBox or Zod). Manifest loader validates before init.
- **Failure isolation.** An in-process plugin that panics can crash
  the daemon. For v1, document this; sidecar isolation is the
  long-term answer for untrusted plugins.

## Consequences

**Makes easier:**

- Multi-runtime adoption (ADR 0018 Phase 2) becomes additive — drop in
  `runtime-hermes` or `runtime-openclaw` without touching daemon code.
- OSS/Pro seam is natural — `plugins/` bundled = OSS; `plugins-contrib/`
  = commercial or community.
- Enterprise deployments swap backends via YAML, not forks.
- Every future concern (cost tracking, federated state, audit export)
  slots into an existing kind or becomes a new kind.
- Plugin authors have one pattern to learn (interface + manifest +
  init) regardless of which kind they target.
- Clean separation: daemon never imports plugin internals.

**Makes harder:**

- Larger surface area: core/ package, manifest schema, registry
  lifecycle, plugin authorship guide all need maintenance.
- Indirection — reading code requires tracing through registry
  lookups, not direct imports.
- Plugin API becomes a versioned stability contract; breaking changes
  have blast radius.
- Initial refactor is substantial (Phase 1 below) — three existing
  ADRs' worth of code moves through registry.

## Phased migration

No big bang. Phase the migration so each step is shippable:

### Phase 1 — Core scaffolding + wrap existing code (no behavior change)

- Create `packages/core` with interfaces, registry, manifest schema,
  lifecycle.
- Wrap existing modules as plugins **in place** (no renaming, no
  functional change):
  - `packages/daemon/src/pi/` → exposed as `runtime-pi` via the registry
    but code stays where it is (move in Phase 2).
  - `packages/daemon/src/memory/` → exposed as `memory-sqlite` plugin.
  - Same for artifacts, secrets, sessions.
- Daemon reads `fleet/plugins.yaml` (with all-defaults allowed if
  absent). Registry initialized at startup.
- No `plugins/` directory yet — plugins are "logical" inside daemon
  package.

**Shippable outcome:** everything still works. Registry seam exists.

### Phase 2 — Extract plugins to their own packages

- Create `packages/plugins/` directory.
- Physically move each wrapped module to its own workspace package.
- Daemon stops importing from `daemon/src/pi/`, etc. — only from
  registry.
- Delete dead imports. Fix any circular dependencies that surface.

**Shippable outcome:** daemon package shrinks. Plugins stand alone.
Repo structure matches the ADR.

### Phase 3 — First contrib plugin to prove the seam

- Pick one contrib plugin (suggested: `memory-postgres`) and implement
  it end-to-end.
- Validates that the registry + contracts + manifest actually support
  alternative implementations without modification to daemon or
  existing plugins.
- Publish the plugin author's guide.

**Shippable outcome:** demonstrated swap in production. Confidence
that future contrib plugins follow the same path.

### Phase 4 — Adopt additional runtime (tied to ADR 0018 Phase 2)

- Implement `runtime-hermes` (or `runtime-openclaw`) as a contrib
  plugin, complete with bridges.
- First fleet running mixed-runtime agents.
- Validates the runtime tier + bridge pattern.

**Shippable outcome:** multi-runtime fleets work. ADR 0018 Phase 2
unblocked.

### Phase 5 — Sidecar isolation, hot reload, plugin SDK

- Add sidecar transport for gateway/mcp-host plugins.
- Publish `@zund/plugin-sdk` for external authors.
- Hot reload (reload without daemon restart) — additive, not
  load-bearing.

**Shippable outcome:** external ecosystem is possible.

## Implementation notes

- `@zund/core` is a workspace package; every other package depends on
  it.
- `ZundContext` passed to `init()` provides: logger, event bus
  (bootstrap), secret resolver, config validator. Do not leak daemon
  internals through context.
- Bridges in runtime plugins follow the convention:
  `packages/plugins/runtime-<name>/src/bridges/<kind>-bridge.ts`.
  One file per service kind the runtime consumes.
- Manifest files co-located with plugin source:
  `packages/plugins/<plugin>/plugin.yaml` (or derived from TS export).
- Tests: each plugin imports `@zund/core/contracts/testkit` and runs
  the contract test suite. Integration tests in daemon use registry
  lookups, not direct imports.
- Backward compatibility: if `fleet/plugins.yaml` is absent, daemon
  uses a default manifest binding all bundled OSS defaults. Existing
  fleets keep working.
- Related ADRs — on acceptance of this ADR, amend:
  - ADR 0003: update to reference the Plugin interface + bridge pattern.
  - ADR 0015, 0016: mark as subsumed by this ADR once Phase 1 lands.
  - ADR 0019: MCP host becomes a plugin kind per this structure.

## Next steps

- Review and lock architectural choices in this ADR.
- Spike Phase 1 core/ scaffolding against a single plugin (memory)
  end-to-end before generalizing.
- Publish the contracts/ directory as the stability contract — any
  change to contract interfaces is a breaking SDK version bump.
- Update `docs/reference/architecture.md` component map to reflect
  the core/plugins split once Phase 2 lands.
