# ADR 0020 Phase 1 — Post-implementation review

Status: **review complete, corrections merged**
Date: 2026-04-17
Branch: `feat/plugin-architecture`
Reviewer: external
ADR status at review time: `accepted` ([0020](../reference/decisions/0020-plugin-architecture-two-tier.md))

## Why this file exists

The branch landed ADR 0020 Phase 1 in ten steps and flipped the ADR status to
`accepted` (commit `1e6dae9`). A separate review found a set of compile
errors, a runtime bug, and two gaps against the ADR's own Phase 1 text. This
file captures what was fixed in this pass, what the implementing developer
pushed back on, and what the reviewer concluded. When Phase 1 ships, this
file graduates to `archive/` alongside [`current.md`](current.md).

## What the review found

Two kinds of issues:

**Compile + runtime bugs (no opinion required):**

1. `packages/core/src/index.ts` re-exported `PluginBinding` and
   `PluginManifest` from `./plugin.ts`; those types live in `./manifest.ts`.
2. `packages/core/src/plugin.ts` used `ZundContext` in the `init()` signature
   before its type-only re-export at the bottom of the file.
3. `packages/core/src/manifest.ts` used a hand-rolled YAML parser with
   five strict-null errors and no support for the inline-object config the
   ADR's example shows (`url: { secret: POSTGRES_URL }`).
4. `packages/daemon/src/api/secrets-routes.ts` — `handleReveal` referenced
   `state.secretStore` but `state` was never a parameter. Any
   `GET /v1/fleet/:name/secrets/:key` would throw a `ReferenceError`.
5. `LocalArtifactStore implements ArtifactStore` (TS2420) — the contract
   widened to include metadata methods in Step 7, but `LocalArtifactStore`
   only covers blob ops.
6. `ArtifactMeta` JSDoc contradicted the type and the runtime: comment said
   `id` is a UUID distinct from the blob sha, but the type has no sha field
   and `DaemonArtifactStore.expiredArtifacts` maps `{ id: meta.id, sha256:
meta.id }`.

**Phase-1 gaps against the ADR's own spec:**

ADR 0020 Phase 1 says (§Phased migration, items 1–3):

> Wrap existing modules as plugins **in place** (no renaming, no functional
> change): `packages/daemon/src/pi/` → exposed as `runtime-pi` via the
> registry but code stays where it is (move in Phase 2).
>
> Daemon reads `fleet/plugins.yaml` (with all-defaults allowed if absent).
> Registry initialized at startup.

Neither was done. The Pi runtime was never registered, and the manifest
loader shipped with zero call sites. `PluginRegistry.runtime("pi")` and
`parseManifest` were public API with no consumer — the easiest class of
mistake to ship and the hardest to undo later.

Additional smaller gaps: `ServiceContracts` type missing `secrets?` after
Step 8 added the contract; `init()` defined on every plugin but never
invoked; "convenience" fields on `AppState` (`memoryStore`, `artifactStore`,
`secretStore`) shadow every `registry.service(kind)` lookup, making the
registry bookkeeping-only; TS errors in `registry.test.ts` and
`plugin-registry.test.ts`.

## What got fixed in this pass

Four subagent waves landed on top of the branch:

**Wave 1a (core compile + YAML + secrets bug):**

- `packages/core/src/index.ts` — re-exports routed through the correct
  module.
- `packages/core/src/plugin.ts` — explicit `import type { ZundContext }`.
- `packages/core/src/manifest.ts` — `simpleYamlParse` deleted, replaced with
  `YAML.parse` from the `yaml` package (added to `@zund/core` deps at the
  same version the daemon uses).
- `packages/daemon/src/api/secrets-routes.ts` — `handleReveal` now takes
  `state: SecretsRouteState`, and the single caller passes it.

**Wave 1b (artifact contract split):**

- `packages/core/src/contracts/artifacts.ts` — extracted `BlobStore` (blob
  ops only) as a base interface; `ArtifactStore` now `extends BlobStore`
  and adds metadata methods. `ArtifactMeta.id` doc reconciled to "sha256
  hex of the blob content".
- `packages/core/src/index.ts` — `BlobStore` re-exported.
- `packages/daemon/src/artifacts/store.ts` — `LocalArtifactStore implements
BlobStore` (correct — it has no metadata methods).
- `packages/daemon/src/artifacts/daemon-store.ts` — blobs dependency typed
  as `BlobStore` rather than the concrete `LocalArtifactStore`.

**Wave 2a (cosmetic fixes the developer agreed to):**

- `ServiceContracts` in `packages/core/src/contracts/runtime.ts` now
  includes `secrets?: SecretStore`. (This had in fact been added already;
  verified.)
- JSDoc in `packages/core/src/plugin.ts` and `packages/core/src/registry.ts`
  flagged that Phase 1 intentionally skips `init()` and Phase 2 will invoke
  it via the manifest loader. Same message in both places so plugin authors
  hit it regardless of where they enter the code.
- `AppState` in `packages/daemon/src/api/server.ts` — single block comment
  explaining the convenience-field pattern is Phase 1 only, Phase 2 removes
  them, new plugin kinds go through the registry.
- TS test errors — `<FakeService>` type arg added to `service()` call sites
  in `packages/core/test/unit/registry.test.ts` (7 lines) and
  `packages/daemon/test/unit/plugin-registry.test.ts` (2 lines). The
  smaller diff than widening the helper; the `register` signature is
  unchanged (public API).

**Wave 2b (thin Phase-1 seam activation — per the ADR's own Phase 1 text):**

- New file `packages/daemon/src/agents/runtimes/pi/runtime.ts` — `PiRuntime`
  class implementing the `Runtime` contract as a thin wrapper:
  `launch`/`stop` throw "not yet wired" errors pointing at Phase 2,
  `session` returns `null`, `mountBridges` is an empty body, `events()` is
  an empty async generator. Plus `createPiRuntimePlugin()` factory.
- `packages/daemon/src/api/server.ts` — registers `runtime:pi` as the
  fourth plugin, and loads `<zundHome()>/plugins.yaml` (host-level
  manifest) at startup. Manifest is **informational in Phase 1**: the
  loader logs source + binding count, and warns on drift between the
  declared bindings and the four hardcoded registrations. No behavior
  change — Phase 2 flips the loader to drive registration.
- `packages/core/src/manifest.ts` — `DEFAULT_BINDINGS` already matched the
  four registrations; verified.
- New tests — `packages/daemon/test/unit/pi-runtime.test.ts` covers every
  `Runtime` method + the plugin factory; manifest test expanded with the
  four-binding default and full-parse assertions.

Final state:

- **Tests:** 451 pass, 0 fail (was 410 pre-review; 41 new, all green).
- **TypeScript:** `@zund/core/src` is clean. `daemon/src/` still has 11
  pre-existing strictness errors in `server.ts` and `artifacts-routes.ts`
  (cast-from-`AgentResource|null`, `req.json()` returning `unknown`,
  undici-types `FormData` leak from `experiments/`). These predate the
  plugin work and were flagged as out-of-scope cleanup in ADR 0003.
- **Boot:** daemon logs four `registered plugin` lines +
  `loaded plugin manifest { source: "default", bindings: 4 }` +
  no drift warning.

## Developer pushback — and the reviewer's read

The implementing developer pushed back on several items, arguing they were
ADR 0003 / Phase 2 scope. Four of those push-backs the reviewer agrees
with; two the reviewer disagrees with. Recorded for posterity.

| Item                                                                                                                    | Dev's position                          | Reviewer's position                                                                                                             | Outcome                                                                          |
| ----------------------------------------------------------------------------------------------------------------------- | --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| Register `runtime:pi` in the registry                                                                                   | "ADR 0003 scope, Phase 2"               | ADR 0020 Phase 1 text literally says "exposed as `runtime-pi` via the registry". Thin wrapper is ~30 lines, no behavior change. | **Done in Wave 2b** — thin wrapper, no executor migration.                       |
| Wire `parseManifest` into startup                                                                                       | "Phase 2 — needs fleetDir at init time" | Host-level `~/.zund/plugins.yaml` has no fleetDir dependency and matches the ADR's example. Informational loader is zero risk.  | **Done in Wave 2b** — informational only, Phase 2 flips to driving registration. |
| Full executor migration (AgentHandle.rpcSession → session, mountBridges implemented, all launches via registry.runtime) | "ADR 0003 execution, Phase 2"           | Agreed. Touches CLI + console; genuinely cross-cutting.                                                                         | **Deferred.**                                                                    |
| `sessionIndexer` as a plugin                                                                                            | "Future concern"                        | Agreed. Not in ADR 0020 scope.                                                                                                  | **Deferred.**                                                                    |
| Shared `MemoryDb` between memory and artifacts plugins                                                                  | "Phase 1 known limitation"              | Agreed. Shipped-as-documented.                                                                                                  | **Deferred.**                                                                    |
| "Registry bookkeeping-only" smell                                                                                       | "Comment in AppState"                   | Comment is acceptable for Phase 1 given the no-behavior-change constraint; Phase 2 removes the convenience fields.              | **Comment landed in Wave 2a; Phase 2 work tracked below.**                       |
| `init()` defined but never called                                                                                       | "Add a caveat comment"                  | Comment is a patch, not a fix — but registry-calls-init is Phase 2 work by design.                                              | **Comment landed in Wave 2a; Phase 2 work tracked below.**                       |

The reviewer's overall take: the developer's instinct to minimize scope is
healthy, but the Phase 1/2 line was being drawn to justify what had
already shipped rather than to satisfy what the ADR committed to. The
remediation in this pass keeps the "no behavior change" principle (every
edit is informational or documentation-only, except the `runtime:pi`
registration which activates a seam the ADR's own Phase 1 required) while
making "Phase 1 accepted" an honest claim.

## Recommended before declaring Phase 1 accepted (all done)

- [x] Pi runtime registered via the registry (thin wrapper, no executor
      migration).
- [x] `parseManifest` wired into startup, warn-on-drift mode.
- [x] `DEFAULT_BINDINGS` in `core/src/manifest.ts` matches the four
      registrations.
- [x] `BlobStore` / `ArtifactStore` split; `LocalArtifactStore implements
  BlobStore`; `ArtifactMeta` docs reconciled.
- [x] `ServiceContracts` gained `secrets?`.
- [x] JSDoc on `ZundPlugin.init` + `PluginRegistry.register` calls out the
      Phase 1 `init()` skip.
- [x] `AppState` convenience-fields pattern documented as Phase 1 only.
- [x] TS test errors fixed.
- [x] Secrets reveal runtime bug fixed.

With these done, the ADR status of `accepted` holds honestly.

## Phase 2 scope (tracked here so Phase 1 isn't re-litigated)

These are all **legitimately** Phase 2 — the reviewer agrees:

- **Physical extraction of `packages/plugins/`** — move the four in-place
  wrappers out of `packages/daemon/` into their own workspace packages
  (ADR 0020 §Phased migration, Phase 2). `@zund/core` remains the
  contract package; daemon imports only through the registry.
- **Manifest-driven registration.** Replace the four hardcoded
  `registry.register(...)` calls in `createState` with a loop that reads
  `manifest.bindings`, resolves each `requires:` dependency graph,
  imports the plugin module, calls `plugin.init(config, ctx)`, and
  registers the result. Per-fleet `<fleetDir>/plugins.yaml` overrides
  layer on top of the host-level manifest.
- **`init()` actually invoked.** Today the registry is passed
  pre-constructed instances. Phase 2's loader builds instances by calling
  `plugin.init(config, ctx)` — at which point every plugin's `init`,
  `requires`, `shutdown`, and `health` hooks become load-bearing.
- **Routes consume plugins through the registry only.** Remove
  `state.memoryStore`, `state.artifactStore`, `state.secretStore`;
  replace each usage with `state.registry.service<T>(kind)`. This is the
  step that finally retires the "convenience fields" caveat from
  `AppState`.
- **Full `Runtime` implementation.** `PiRuntime.launch` / `stop` actually
  delegate to `launchAgent` / `destroyAgent`; `session` returns the
  `RuntimeSession` interface over `AgentRpcSession`; `mountBridges` wires
  memory/artifacts/secrets into Pi's tool registry per ADR 0020
  §"The runtime tier — bridges"; `events()` yields from the active RPC
  sessions.
- **`AgentHandle` reshape** — `rpcSession: AgentRpcSession` →
  `session: RuntimeSession`. Touches CLI + console transport types.
- **`AgentResource.runtime` field** — per ADR 0003 Consequences. Default
  `"pi"`; registry dispatches through it.
- **Stream translator** — move `translateEvent` into a per-runtime map
  keyed by `runtime.name`, drop the hardcoded `"pi"` argument in
  `server.ts`.

## Items explicitly out of scope through Phase 2

- Pre-existing strictness errors in `server.ts` + `artifacts-routes.ts`
  (cast-from-`AgentResource|null`, `req.json()` returning `unknown`,
  `FormData` type leak from `experiments/`). Track as a separate cleanup
  pass — flagged in [ADR 0003](../reference/decisions/0003-agent-runtime-interface.md)
  §"Designs for ADR 0020".
- `sessionIndexer` becoming a plugin — not in ADR 0020 at all.
- Splitting the shared `MemoryDb` between the memory and artifacts plugins
  — documented as a Phase 1 limitation in the original plan; natural to
  resolve when artifacts moves to its own package (Phase 2).

## Files touched in this review pass

**Created:**

- `packages/daemon/src/agents/runtimes/pi/runtime.ts`
- `packages/daemon/test/unit/pi-runtime.test.ts`
- `docs/roadmap/adr-0020-phase1-review.md` (this file)

**Edited:**

- `packages/core/package.json` (added `yaml` dep)
- `packages/core/src/contracts/artifacts.ts`
- `packages/core/src/index.ts`
- `packages/core/src/manifest.ts`
- `packages/core/src/plugin.ts`
- `packages/core/src/registry.ts` (JSDoc only)
- `packages/core/test/unit/manifest.test.ts`
- `packages/core/test/unit/registry.test.ts`
- `packages/daemon/src/api/secrets-routes.ts`
- `packages/daemon/src/api/server.ts`
- `packages/daemon/src/artifacts/daemon-store.ts`
- `packages/daemon/src/artifacts/store.ts`
- `packages/daemon/test/unit/plugin-registry.test.ts`
