# ADR 0020 Phase 2 — Roadmap

Status: **done** (sub-slices 2a + 2b + 2c + 2d + 2e shipped)
Date: 2026-04-17
Branch: `feat/plugin-architecture`
Related: [ADR 0020](../reference/decisions/0020-plugin-architecture-two-tier.md), [Phase 1 review](adr-0020-phase1-review.md)

## Why this file exists

Phase 2 of ADR 0020 is large — ~3,200 lines of plugin code moves between packages, the Pi runtime gets a real `Runtime` implementation, the manifest loader becomes load-bearing, and Pi's in-container tool registrations get reorganized into per-kind bridges. Doing it all in one commit would be a big bang; this doc slices it into five shippable sub-slices so any future fresh session can pick up from any one of them without re-exploring the codebase.

This doc graduates to `archive/` alongside the Phase 1 review when Phase 2 is fully done.

## Current state at time of writing

Phase 1 (commit `1e6dae9` + four review waves) shipped the `PluginRegistry` seam inside `packages/daemon/`. The daemon registers four plugins (`memory:sqlite`, `artifacts:local`, `secrets:age-sops`, `runtime:pi`) but:

- Routes still read `state.memoryStore` / `state.artifactStore` / `state.secretStore` convenience fields that shadow the registry. **Sub-slice 2a retires these.**
- `PiRuntime` is a thin stub: `launch`/`stop` throw, `session` returns `null`, `mountBridges` is empty. **Sub-slice 2b activates it.**
- `plugin.init(config, ctx)` is defined on every plugin but never invoked — the daemon instantiates concrete classes directly. **Sub-slice 2c wires the loader.**
- The four plugins live inside `packages/daemon/src/{memory,artifacts,secrets,agents/runtimes/pi}/`. **Sub-slice 2d moves them to `packages/plugins/*`.**
- Pi's tool registrations (`memory_save`, `zund_emit_artifact`, etc.) live in `extension.ts` monolithically. **Sub-slice 2e splits them into per-kind bridges.**

## Sub-slice map

| # | Name | Status | Depends on | Ships when |
|---|------|--------|------------|------------|
| 2a | Convenience-field retirement | shipped | — | `grep` for `state.memoryStore` returns zero, all tests pass |
| 2b | Full Pi Runtime seam | shipped (Option B host injection) | 2a | `handleAgentMessage` routes through `registry.runtime().session().message()`, no hardcoded `"pi"` |
| 2c | Manifest-driven registration | shipped | 2a | `createState`'s four hardcoded register blocks replaced with a loader loop; pulled forward the MemoryDb/artifacts split and `ZundContext.paths` from 2d for a clean `init()` contract |
| 2d | Physical extraction to `packages/plugins/` + extension.ts bridge file split | shipped | 2c | Four workspace packages live under `packages/plugins/`; daemon has zero imports into old plugin dirs; extension.ts split into bridges; `ZundContext.{log,incus}` added; fleet types moved to `@zund/core/contracts/fleet`; all 430+ plugin tests pass; identity-checked bridge output |
| 2e | Registry-driven mountBridges | shipped | 2d | `generateExtension` reads `ExtensionConfig.boundKinds` (snapshot of `registry.boundKinds()`); memory/artifacts bridges only emitted when bound |

Sub-slices 2b and 2c are independent — either can land first after 2a.

> **2b notes:** shipped without migrating the executor's launch/destroy paths.
> `PiRuntime.launch`/`stop` still throw — the executor calls
> `launchAgent`/`destroyAgent` directly. The route migration (`handleAgentMessage`
> via `registry.runtime(name).session(name)`) is what 2b actually wires.
> `PiRuntimeHost` constructor injection is the Option B stopgap; sub-slice 2c
> will route this through `init(config, ctx)`. Out-of-route `rpcSession`
> accesses (health check, `new_session` route, debug endpoint, reconciler) are
> deliberately unchanged.

> **2c notes:** shipped with two 2d items pulled forward by design:
>
> - **MemoryDb/artifacts split.** Artifacts now owns `<zundData>/artifacts.db`
>   via a new `ArtifactMetaDb` class, with a first-run migration that copies
>   `artifact_meta` rows from any legacy `memory.db`. The artifacts plugin's
>   `init()` is now fully self-contained — no shared `MemoryDb` construction
>   in `createState`. 2d no longer has to unbundle this coupling.
> - **`ZundContext.paths`.** Added `{ home, data, artifacts }` to the context
>   so plugin `init()` methods resolve their files from context instead of
>   closure-captured daemon paths. Factory closure is still used for non-path
>   daemon state (the live `agents` map for `runtime:pi`).
>
> The four factories (`createMemorySqlitePlugin`, `createArtifactsLocalPlugin`,
> `createSecretsAgeSopsPlugin`, `createPiRuntimePlugin`) now live next to their
> implementations under `packages/daemon/src/{memory,artifacts,secrets,agents/runtimes/pi}/plugin.ts`
> (or `runtime.ts`). The new loader (`@zund/core/loader`) topo-sorts bindings by
> `requires`, calls `init(config, ctx)` per binding, and treats a manifest
> entry without a registered factory as a hard error (Phase 1's drift warning
> is gone). `server.close()` → `registry.shutdownAll()` is the same path as
> before, but the daemon's SIGTERM handler now awaits it before `process.exit(0)`
> instead of cutting it short.
>
> Deferred from 2c: per-fleet `<fleetDir>/plugins.yaml` overrides — `fleetDir`
> isn't known at boot and re-initializing plugins on `apply` opens a lifecycle
> question that should land with 2d's workspace extraction, not here.
> `ZundContext.incus` likewise stays out — runtime-pi is the only consumer and
> the factory-closure host is simpler.

> **2d notes:** shipped with the extension.ts bridge split pulled forward
> from 2e by design (file moved + split in one pass, not move-then-split).
>
> - **Four workspace packages.** `@zund/plugin-memory-sqlite`,
>   `@zund/plugin-artifacts-local`, `@zund/plugin-secrets-age-sops`,
>   `@zund/plugin-runtime-pi` now live under `packages/plugins/`. Each
>   package's `init()` already read from `ctx.paths` post-2c, so no plugin
>   logic changed — only file locations and import paths.
> - **Fleet types in core.** `AgentResource`, `ArtifactPolicy`, `SecretRef`,
>   and the rest of the resource hierarchy moved to
>   `@zund/core/contracts/fleet.ts`. Daemon's `fleet/types.ts` re-exports
>   from core so daemon-internal imports keep working.
> - **ZundContext additions.** `log: (scope) => ZundLogger` plus
>   `incus: ZundIncusFacade` (narrow facade: `execInContainer`,
>   `mountHostDir`, `createFromBaseImage`, `setContainerConfig`,
>   `stopAndDelete`). Daemon `createState` constructs the facade over its
>   real `IncusClient`. Runtime-pi's `launcher`/`config`/`extension-writer`
>   call through the facade — no `IncusClient` type leak into plugins.
> - **Logger threading.** `launchAgent`, `destroyAgent`, `writeExtension`,
>   and `startArtifactSweeper` now take `log: ZundLogger` as a parameter.
>   Callers (executor, server) pass their scoped loggers in. No plugin
>   imports daemon's `createLogger` anymore.
> - **Bridge split.** `extension.ts` (592 lines) split into orchestrator +
>   `bridges/{fleet-bridge, memory-bridge, artifacts-bridge}.ts`. Sub-generators
>   are concatenated unconditionally for now; registry-driven conditional
>   inclusion lands in 2e. Identity-checked: byte-diff against the original
>   pre-split generator is zero on both teamed and solo configs.
> - **Sweeper path helper.** `zundArtifactBlob` (previously imported from
>   daemon's `paths.ts`) inlined into `artifacts-local/src/store.ts` as
>   `blobPath(root, sha)`, with the root passed in via the plugin's
>   `LocalArtifactStore` constructor.
> - **Executor seam.** `ExecutorState` gained `incusFacade: ZundIncusFacade`
>   alongside `incusClient: IncusClient`; executor calls `launchAgent`,
>   `destroyAgent`, `writeExtension` with the facade. Server's direct
>   `destroyAgent` calls (fleet delete path) use the facade + server's
>   `createLogger("api")` instance.
> - **Tests.** 430+ plugin tests pass across the four packages. Daemon
>   unit suite: 176 pass. HTTP integration suite: 62 pass. Smoke-booted
>   a fresh `ZUND_HOME` — all four plugins register via the manifest and
>   `/health` responds.
>
> Deferred to 2e: per-fleet `<fleetDir>/plugins.yaml` overrides still not
> wired (same reasoning as 2c).

> **2e notes:** shipped as a narrow seam — `generateExtension` now takes a
> `boundKinds: readonly PluginKind[]` snapshot on `ExtensionConfig` and only
> emits the memory and artifacts bridges when their kinds are bound. Fleet
> tools (`zund_fleet_status`, `load_skill`) and the `before_agent_start` /
> `session_start` handlers always emit — fleet isn't a plugin kind.
>
> - **Registry**: added `PluginRegistry.boundKinds(): PluginKind[]` returning
>   each kind with ≥ 1 registered instance. Deduped across named alternatives
>   (e.g. `memory:sqlite` + `memory:postgres` → `["memory"]`).
> - **ExecutorState snapshot**: `createState` captures `registry.boundKinds()`
>   once at boot into `state.boundKinds`. The registry is immutable post-load,
>   so a snapshot is safe and avoids leaking the full registry through the
>   executor seam. All three `writeExtension` callsites pass `state.boundKinds`.
> - **Extension generator gating**: the `inferMimeType` helper is only emitted
>   when `artifacts` is bound (it's the sole caller). With `boundKinds=[]` the
>   generated file is fleet-tools-only (~240 lines shorter).
> - **Tests**: new `extension-bound-kinds.test.ts` asserts conditional
>   inclusion across four configs (memory-only, artifacts-only, fleet-only,
>   unrelated-kinds). Existing 39 extension tests updated to pass
>   `boundKinds: ["memory", "artifacts"]` matching prior behavior.
> - **Smoke**: fresh `ZUND_HOME` boots, `/health` responds. All test suites
>   pass (core 43 / plugin 261 / daemon unit 176 / daemon integration 66).
>
> Deferred: making the fleet-bridge's prompt advertising dynamic (today the
> `before_agent_start` handler always lists "memory_save, memory_search,
> working_memory_update, working_memory_patch, zund_fleet_status, load_skill,
> zund_emit_artifact" in the injected Tools line regardless of bound kinds).
> Today's four-plugin default covers all listed tools so this is cosmetic;
> fix when someone actually runs with a reduced plugin set. Per-fleet
> `<fleetDir>/plugins.yaml` overrides also still deferred (3rd time in a row;
> wait for a real user demand before wiring the lifecycle question).

---

## Sub-slice 2b — Full Pi `Runtime` seam (daemon-internal)

### Goal

Make `PiRuntime` actually implement `Runtime`. Daemon routes invoke Pi through the registry (`state.registry.runtime(agent.runtime ?? "pi")`) instead of reaching into `AgentRpcSession` directly. **No CLI or console changes** — CLI only touches HTTP endpoints, not `AgentHandle` / `AgentRpcSession` types.

### What's there today (verified via explore)

| Thing | Location | Shape |
|-------|----------|-------|
| `launchAgent` | `packages/daemon/src/agents/runtimes/pi/launcher.ts:58` | `(client, config: LaunchAgentConfig) => Promise<AgentHandle>` |
| `destroyAgent` | `packages/daemon/src/agents/runtimes/pi/launcher.ts:170` | `(client, handle) => Promise<void>` |
| `AgentHandle` | same file lines 21–30 | `{ name, containerName, rpcSession: AgentRpcSession, status, ... }` |
| `AgentRpcSession` | `packages/daemon/src/agents/runtimes/pi/rpc.ts:32` | methods: `start/stop/sendCommand/sendPromptStreaming/alive/isActive` |
| `RuntimeSession` contract | `packages/core/src/contracts/runtime.ts:50` | methods: `message(payload): AsyncIterable<RuntimeEvent>`, `close()` |
| `translateEvent` | `packages/daemon/src/stream/translator.ts:22` | `(runtimeName, event) => CanonicalEvent`; today identity, hardcoded `"pi"` at `server.ts:693` |
| `AgentResource` | `packages/daemon/src/fleet/types.ts:78` | NO `runtime` field today |

### Plan shape

1. **Adapter: `PiRuntimeSession implements RuntimeSession`** (new file `packages/daemon/src/agents/runtimes/pi/session.ts`). Wraps `AgentRpcSession`. `message(payload)` calls `rpcSession.sendPromptStreaming(payload.text)` and re-yields events through `translateEvent("pi", event)`. `close()` calls `rpcSession.stop()`.
2. **`PiRuntime.launch(agent, ctx)`** (in `runtime.ts`) delegates to `launchAgent(state.incusClient, buildLaunchConfig(agent, ctx))`, stores handle in `state.agents`, returns an `AgentResourceRef` (see contract).
3. **`PiRuntime.stop(ref)`** looks up `state.agents.get(ref.name)` and calls `destroyAgent(state.incusClient, handle)`.
4. **`PiRuntime.session(agentName)`** looks up `state.agents.get(name)`; if present and `rpcSession.alive`, returns `new PiRuntimeSession(rpcSession)`; else `null`.
5. **`PiRuntime.events()`** multiplexes events from active `state.agents`. Use a `ReadableStream` merge or async iterator that subscribes to each session's event source. Details to firm up when touching rpc.ts.
6. **`AgentResource.runtime?: string`** field added in `fleet/types.ts`. Default `"pi"` applied in `fleet/defaults.ts`. Update validator.
7. **Per-runtime translator**: extract translation logic into `packages/daemon/src/agents/runtimes/<name>/translator.ts`. `stream/translator.ts` becomes a dispatcher keyed by runtime name. Call site `server.ts:693` drops the hardcoded `"pi"` and uses `agent.runtime`.
8. **Route migration**: `handleAgentMessage` in `packages/daemon/src/api/server.ts` around line 644 — today reads `handle.rpcSession.sendPromptStreaming(message)`. Change to `const runtime = state.registry.runtime(agent.runtime); const session = runtime.session(agent.name); for await (const event of session.message({text: message})) { ... }`.

### Constructor wiring note

`PiRuntime` needs access to `state` (specifically `state.agents` and `state.incusClient`) to do its job. Options:
- **Option A:** Pass context via `ctx: ZundContext` on `init` — but Phase 1 still calls `new PiRuntime()` directly. Once 2c lands, this becomes natural.
- **Option B:** Make `PiRuntime` a factory that closes over `state`. Less pure but works today.

Recommend **Option A** if 2c lands before 2b (good reason to order 2c before 2b). Otherwise Option B as a stopgap.

### Verification

- Boot daemon, apply a fleet, send a message via `POST /v1/agents/:name/message`. Should stream events exactly as today.
- `grep -n '"pi"' packages/daemon/src/api/server.ts` → no hardcoded runtime argument.
- `grep -n "rpcSession" packages/daemon/src/api/server.ts` → ideally zero direct accesses; routes go through the runtime.
- Unit tests for `PiRuntimeSession` (new file) — wrap a fake `AgentRpcSession`, assert `message()` yields translated events and `close()` calls `stop()`.
- Existing `pi-runtime.test.ts` needs updates: the "throws not yet wired" assertions for `launch`/`stop` flip to real lifecycle tests. Mock `launchAgent`/`destroyAgent` or test against an integration harness.

### Out of scope for 2b

- Anything inside `packages/plugins/` (physical moves = 2d).
- `init()` invocation (= 2c).
- `mountBridges` content (= 2e).

---

## Sub-slice 2c — Manifest-driven registration

### Goal

The manifest (`~/.zund/plugins.yaml` + optional `<fleetDir>/plugins.yaml`) drives which plugins register. Each plugin's `init(config, ctx)` builds the instance. `requires`, `shutdown`, and `health` hooks become load-bearing. The Phase 1 drift warning becomes a hard error.

### What's there today

- `PluginManifest` parsed from YAML: `packages/core/src/manifest.ts:88 (parseManifest)`. Shape: `{ bindings: PluginBinding[], config: Record<string, unknown> }`. Default returned when no file exists.
- Phase 1 loader in `packages/daemon/src/api/server.ts:110-127` reads the manifest but only for drift detection. Four hardcoded `registry.register(...)` blocks follow at lines 137-181.
- `ZundPlugin.init(config, ctx) => Promise<Instance>` already in the type. Phase 1 factories return the pre-built instance.
- `ZundContext` at `packages/core/src/context.ts` — shape: `{ logger, registry, secretResolver, ... }` (verify exact shape when starting).

### Plan shape

1. **Each plugin factory rebuilds under `init`.** E.g.:
   ```ts
   export function createMemorySqlitePlugin(): ZundPlugin<"memory", MemoryStore> {
     return {
       kind: "memory", name: "sqlite", version: "0.1.0", provides: ["memory:sqlite"],
       async init(config: { path?: string }, ctx: ZundContext) {
         const path = config.path ?? join(zundData(), "memory.db");
         return new SqliteMemoryStore(new MemoryDb(path));
       },
       async shutdown() { /* close db */ },
     };
   }
   ```
2. **Loader loop** in `createState`: replace the four hardcoded register blocks with a loop:
   ```ts
   for (const binding of topoSort(manifest.bindings, getPlugin)) {
     const plugin = await loadPlugin(binding);
     const config = manifest.config[binding.name] ?? {};
     const instance = await plugin.init(config, ctx);
     registry.register(plugin, instance);
   }
   ```
3. **Plugin resolution.** A static map of bundled plugins today: `{ "memory:sqlite": createMemorySqlitePlugin, "artifacts:local": ..., "secrets:age-sops": ..., "runtime:pi": createPiRuntimePlugin }`. After 2d, this becomes dynamic `import()` by package name.
4. **Topological sort over `requires`.** Simple Kahn's algorithm. Fail fast with a clear error on cycles.
5. **Per-fleet overrides.** When `<fleetDir>/plugins.yaml` exists, parse it too. Merge strategy (clearest): per-fleet entries replace host entries for the same `kind`. Document in the loader with an example.
6. **Drift becomes error.** Remove the warn-only drift log — if the manifest declares a plugin the system can't load, daemon fails to start.
7. **`shutdownAll` call path.** `registry.shutdownAll()` already exists. Wire it into daemon SIGTERM handling.

### Verification

- Boot with no `~/.zund/plugins.yaml` — uses defaults, all four plugins load via `init()`, daemon runs normally.
- Boot with a `~/.zund/plugins.yaml` that declares the same four plugins with custom config (e.g., memory path) — config reaches `init()`, instance uses overridden path.
- Boot with a manifest that declares a nonexistent plugin — daemon fails to start with a clear error.
- `grep -n "registry.register" packages/daemon/src` → zero explicit calls; all go through the loader.
- Existing tests for the registry and manifest stay green; add a loader-loop test.

### Out of scope for 2c

- Moving files (= 2d).
- Dynamic `import()` from npm packages — defer to ADR 0020 Phase 3 (first contrib plugin).

---

## Sub-slice 2d — Physical extraction to `packages/plugins/`

### Goal

Move the four in-place wrappers into their own workspace packages under `packages/plugins/`. Daemon imports zero files from `packages/daemon/src/{memory,artifacts,secrets,agents/runtimes/pi}/{store,db,vault,…}.ts`.

### File inventory (from explore, verbatim)

| Plugin | Source dir today | Files | Line totals |
|--------|------------------|-------|-------------|
| memory-sqlite | `packages/daemon/src/memory/` | `db.ts` (545), `markdown.ts` (110), `store.ts` (112) | 767 |
| artifacts-local | `packages/daemon/src/artifacts/` | `store.ts` (74), `daemon-store.ts` (81), `policy.ts` (126), `sweeper.ts` (87) | 368 |
| secrets-age-sops | `packages/daemon/src/secrets/` | `store.ts` (44), `vault.ts` (117), `resolver.ts` (190), `consumers.ts` (118), `age.ts` (90), `sops.ts` (92) | 651 |
| runtime-pi | `packages/daemon/src/agents/runtimes/pi/` | `runtime.ts` (70), `launcher.ts` (217), `rpc.ts` (370), `extension.ts` (592), `config.ts` (164), `extension-writer.ts` (55) | 1468 |

### Critical couplings to resolve

1. **Shared `MemoryDb`.** `packages/daemon/src/artifacts/daemon-store.ts` imports `MemoryDb` from `../memory/db.ts`. In Phase 2d, artifacts gets its own SQLite file at `<zundData()>/artifacts.db`. One-time migration copies `artifact_meta` rows from `memory.db` to `artifacts.db` on first startup. Ship as a standalone commit *inside* 2d so regressions bisect cleanly.
2. **Fleet types plugins need.** `secrets/resolver.ts` and `secrets/consumers.ts` import `AgentResource`, `SecretRef` from `../fleet/types.ts`. `artifacts/policy.ts` imports `AgentResource`, `ArtifactPolicy`. Move these types to `@zund/core/contracts/fleet.ts`. Daemon `fleet/types.ts` re-exports them. Avoids the circular `plugin → daemon → plugin` import.
3. **Infrastructure plugins need.** Runtime-pi uses `../../../incus/*`, `../../../paths.ts`, `../../../log.ts`. Incus is daemon infrastructure. Plugin imports daemon? Circular risk. Options:
   - Extract `incus`, `paths`, `log` into `@zund/core/utils` (clean, but core grows).
   - Pass them through `ZundContext` at `init(config, ctx)` time (cleaner, but context grows).
   - Plugin depends on daemon workspace package explicitly (simplest; register daemon as `@zund/daemon` and expose utils). Risk: plugin coupling to daemon version.

   Prefer **pass-through via `ZundContext`** — it's what context was designed for. Each new `ctx.incus`, `ctx.paths`, `ctx.log` field is an additive core change with clear semantics.

### Steps

1. Add `packages/plugins/*` to `pnpm-workspace.yaml`.
2. Create four workspace packages. Each gets a minimal `package.json`:
   ```json
   {
     "name": "@zund/plugin-memory-sqlite",
     "private": true,
     "type": "module",
     "dependencies": { "@zund/core": "workspace:*" }
   }
   ```
   No build script; Bun consumes `.ts` directly. Mirror `@zund/core` layout.
3. Move files. Update imports — most internal imports stay path-relative within the plugin. External imports change:
   - `@zund/core/...` stays as-is.
   - `../../../incus/*`, `../../../paths.ts`, `../../../log.ts` become `ctx.incus`, `ctx.paths`, `ctx.log` (push them through context).
   - `../fleet/types.ts` → `@zund/core/contracts/fleet.ts` (after the type move).
4. **Break `MemoryDb` coupling** in its own commit. Create `ArtifactMetaDb` class in the artifacts plugin, backed by `<zundData()>/artifacts.db`. Add first-run migration.
5. Update the plugin loader's static map (from 2c) to `import()` from the new package names.
6. Move co-located unit tests with each plugin. Keep integration / server tests in daemon.

### Test migration

From the explore:
- `test/unit/memory-db.test.ts`, `memory-markdown.test.ts`, `memory-store.test.ts` → `packages/plugins/memory-sqlite/test/unit/`
- `test/unit/artifacts-{policy,store}.test.ts`, `db-artifacts.test.ts`, `sweeper.test.ts` → `packages/plugins/artifacts-local/test/unit/`
- `test/unit/secrets/*.test.ts`, `resolver.test.ts`, `vault.test.ts` → `packages/plugins/secrets-age-sops/test/unit/`
- `test/unit/pi-runtime.test.ts` + other pi tests → `packages/plugins/runtime-pi/test/unit/`
- Stay in daemon: `server.test.ts`, `executor.test.ts`, all of `test/integration/`.

### Verification

- `find packages/daemon/src/{memory,artifacts,secrets} -type f` → empty (or only bridge/interop remnants).
- `grep -rn "from \"\.\.\/memory\|from \"\.\.\/artifacts\|from \"\.\.\/secrets" packages/daemon/src` → zero hits.
- `pnpm install` creates workspace links for the four new packages.
- All tests still pass.
- Daemon boot logs unchanged (same four `registered plugin` lines).
- First-run migration: delete `~/.zund/data/artifacts.db`, boot, confirm metadata gets copied from `memory.db`.

### Pulled forward from 2e: extension.ts bridge file split

When `extension.ts` (592 lines) moves to `packages/plugins/runtime-pi/src/extension.ts`, split it at the same time instead of landing a monolith and re-splitting in 2e. The new layout:

```
packages/plugins/runtime-pi/src/
  runtime.ts
  extension.ts          (orchestrator: assembles generated file from bridge strings)
  bridges/
    memory-bridge.ts    (generateMemoryBridge(config) → string)
    artifacts-bridge.ts (generateArtifactsBridge(config) → string)
    secrets-bridge.ts   (env injection at launch — different shape)
    fleet-bridge.ts     (zund_fleet_status, load_skill)
```

Each bridge exports a generator that returns the `pi.registerTool(...)` template-literal chunks for its kind. `generateExtension(config)` concatenates them unconditionally — conditional inclusion based on `registry.boundKinds()` is 2e.

Rationale for pulling this in: a monolith move followed by a split re-touches the same 592-line file twice. Same pattern as the `MemoryDb`/`ArtifactMetaDb` split pulled into 2c.

### Out of scope for 2d

- Registry-driven conditional bridge inclusion (= 2e).
- Dynamic npm-package plugin discovery (Phase 3).
- `sessionIndexer` → plugin (not in ADR 0020).
- `incus/*` becoming its own workspace package (unless it falls out naturally).

---

## Sub-slice 2e — Registry-driven mountBridges

### Goal

After 2d, per-kind bridge generators already exist under `packages/plugins/runtime-pi/src/bridges/`. 2e makes `generateExtension`/`mountBridges(ctx)` walk the registry and include only the bridges for kinds that are actually bound. Today (post-2d) all four bridges are unconditionally concatenated.

### What's there today (post-2d)

Four bridge generators already live in `packages/plugins/runtime-pi/src/bridges/{memory,artifacts,secrets,fleet}-bridge.ts`. `generateExtension` concatenates all four unconditionally.

### Plan shape

`generateExtension(config, ctx)` — or equivalently `PiRuntime.mountBridges(ctx)` — consults `ctx.registry.boundKinds()` and only includes the bridges for bound kinds. Something like:

```ts
const kinds = ctx.registry.boundKinds();
let out = extensionPrelude(config);
if (kinds.includes("memory"))    out += generateMemoryBridge(config);
if (kinds.includes("artifacts")) out += generateArtifactsBridge(config);
if (kinds.includes("secrets"))   out += generateSecretsBridge(config);
out += generateFleetBridge(config); // always on — fleet not a plugin yet
```

The interesting work is the new `boundKinds()` (or equivalent) on the registry and threading the registry through to the extension generator. Test: apply a fleet with only `artifacts` bound, confirm `memory_save` is not present in the generated extension.

### Verification

- `extension.ts` post-2e is ≤ ~150 lines (orchestrator only — bridge content lives in `bridges/*.ts` since 2d).
- Unit tests for conditional inclusion: call `generateExtension` with a stub registry exposing only `artifacts`, assert output contains `zund_emit_artifact` but not `memory_save`.
- Integration test: boot daemon with a manifest that only declares `artifacts:local`, apply an agent, confirm the on-disk extension file has no memory tools.

### Out of scope for 2e

- Non-HTTP transport (direct plugin calls from container) — needs a daemon↔container API design, not in this ADR.
- New bridge kinds (MCP, bus) — out of ADR 0020 Phase 2.

---

## Decisions locked

- **Lookup style in 2a:** typed helper wrappers `memory(state)` / `artifacts(state)` / `secrets(state)` — not inline `state.registry.service<T>("kind")`.
- **Ordering:** 2a → 2c → 2b → 2d → 2e recommended (2c before 2b so `PiRuntime` can receive context via `init`; 2d after both so loader already exists).
- **Shared MemoryDb split:** artifacts gets its own `<zundData()>/artifacts.db` during 2d. Ship as a standalone commit inside the slice for clean bisect.
- **Fleet types for plugins:** `AgentResource`, `ArtifactPolicy`, `SecretRef` move to `@zund/core/contracts/fleet.ts` during 2d.
- **Infrastructure pass-through:** `incus`, `paths`, `log` flow to plugins via `ZundContext.{incus,paths,log}` — not via direct imports from daemon.
- **CLI/console blast radius:** zero. CLI only touches HTTP; `AgentHandle` / `AgentRpcSession` reshape is daemon-internal (confirmed by explore).

## Items out of scope through Phase 2

- Pre-existing strictness errors in `server.ts` + `artifacts-routes.ts` (cast-from-`AgentResource|null`, `req.json()` returning `unknown`, `FormData` type leak from `experiments/`) — tracked as ADR 0003 cleanup.
- `sessionIndexer` becoming a plugin — not in ADR 0020.
- Sidecar transport for plugins (Phase 5).
- External npm-package plugin discovery (Phase 3).
- Runtimes providing service kinds (Hermes native memory etc.) — Phase 4.

## How to pick this up in a fresh session

1. Check what's committed on `feat/plugin-architecture` vs this doc's "Status" table.
2. Read the sub-slice section for what's next.
3. Re-run the explore queries in the [Phase 1 review](adr-0020-phase1-review.md) if any file inventory looks stale (memories decay — verify before acting on line numbers).
4. Execute. Each sub-slice is shippable alone.
