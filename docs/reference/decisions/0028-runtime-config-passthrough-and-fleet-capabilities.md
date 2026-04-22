---
id: "0028"
title: "Runtime config pass-through and fleet_capabilities contract"
date: 2026-04-18
status: accepted
implementation: partial
supersedes: []
superseded_by: null
related: ["0003", "0013", "0020", "0027", "0029", "0030"]
tags: [runtime, plugins, architecture, capabilities, l2, l3]
---

# 0028 · Runtime config pass-through and fleet_capabilities contract

Date: 2026-04-18
Status: draft
Related: ADR 0003 (runtime interface), ADR 0013 (Pi tools field —
partially superseded by the `fleet_capabilities:` block defined here),
ADR 0020 (plugin architecture — this ADR extends the runtime-tier
contract), ADR 0027 (Pi baseline extensions — the concrete Pi
expression of `fleet_capabilities:` bridges), ADR 0029 (mcporter
sidecar — third tier), ADR 0030 (packs — how bundles plug into
`runtime_config:` and `fleet_capabilities:`).

## Context

ADR 0020 made plugins swappable. The two-tier (runtime + service)
shape held up well for the services it was designed around — memory,
secrets, artifacts, bus, queue. It does not hold up for the **runtime's
own configuration surface**.

Agent runtimes have massive native configuration. A short inventory:

- **Hermes's `~/.hermes/config.yaml`** — six terminal backends (local,
  docker, ssh, singularity, modal, daytona), four web-search backends
  (Exa, Firecrawl, Parallel, Tavily) each with LLM-assisted
  compression options (Gemini / OpenRouter), toolset selectors,
  managed tool-gateway for subscribers, URL safety policy, website
  policy, model aliases.
- **Pi's config** — model defaults, extension lists, built-in tool
  toggles, hook configuration.
- **OpenClaw** — skill registry bindings, MCP server configurations,
  terminal backend, model routing rules.

Trying to abstract all of this into Zund-owned contracts is either
lossy (we drop fidelity) or a massive re-implementation (we rebuild
Hermes's web_tools.py as a Zund plugin kind). Neither is correct. The
runtime author already knows the shape; Zund should get out of the
way.

But the runtime also cannot own **fleet-level state**. The task
queue, the memory store shared across the fleet, the artifact and
docs stores, `fleet_status`, `task-delegate`, and the mcporter sidecar
all depend on fleet-wide resources no single-agent runtime knows
about. Every runtime plugin must bridge to these somehow.

The clean seam is: **Zund owns the fleet primitives every runtime
MUST bridge; the runtime owns its own configuration, piped through
verbatim.**

## Decision

Introduce two new top-level blocks in fleet YAML and extend the
runtime plugin contract to make the seam explicit.

### 1. Two new fleet-YAML blocks

**`runtime_config:`** — opaque to Zund. Written verbatim into the
runtime's native config file at container boot. Secret refs
(`ref://secrets.X`) resolve via the active secrets plugin before
injection. Zund never reads or interprets; it is passthrough.

**`fleet_capabilities:`** — Zund-owned enumeration of which fleet
primitives this agent uses. Each entry resolves to a runtime-plugin-
provided bridge. The v1 vocabulary:

```
memory         — fleet memory store (ADR 0015/0016)
artifacts      — fleet artifact store (ADR 0011; merges into docs per 0025)
docs           — unified docs store (ADR 0025)
fleet-status   — agent discovery surface
task-delegate  — submit tasks to the fleet queue (ADR 0023)
mcp            — shared mcporter sidecar (ADR 0029)
```

The vocabulary grows deliberately — new entries require an ADR
amendment or a new ADR (same discipline as ADR 0023's `hints` keys).

### 2. Runtime plugin contract additions

Extends the `Runtime` interface (ADR 0003) and the runtime-tier
plugin shape (ADR 0020):

```typescript
// packages/core/src/contracts/runtime.ts (additions)

export interface RuntimePlugin extends ZundPlugin<"runtime", Runtime> {
  /**
   * Where the runtime expects its native config inside the container.
   * Pi:      "~/.pi/config.toml"
   * Hermes:  "~/.hermes/config.yaml"
   * OpenClaw:"~/.openclaw/config.yaml"
   */
  configPath: string;

  /**
   * Serialization format for the runtime's native config file.
   * Zund renders `runtime_config:` YAML into this format before
   * writing the file at container boot.
   */
  configFormat: "yaml" | "toml" | "json" | "ini";

  /**
   * Called once per entry in `fleet_capabilities:`. Returns the code
   * or config the runtime needs to wire up that capability.
   *
   * For Pi, this invokes the extension generators (writes TS into
   * `~/.pi/extensions/zund-fleet/<capability>.ts`).
   * For Hermes, this might write plugin entries or skill files.
   * For OpenClaw, this might register a skill and a tool.
   *
   * Each runtime implements its own mechanism. Return success means
   * the capability is live inside the container.
   */
  bridgeFor(
    capability: FleetCapability,
    ctx: BridgeContext,
  ): Promise<BridgeResult>;
}

export type FleetCapability =
  | "memory" | "artifacts" | "docs"
  | "fleet-status" | "task-delegate" | "mcp";

export interface BridgeContext {
  agent: AgentIdentity;
  registry: ServiceRegistry;   // for resolving the bound service plugin
  containerFs: ContainerFs;    // for writing generated code / configs
  secrets: SecretResolver;
  daemonEndpoint: string;      // for bridges that call back (task-delegate)
}

export interface BridgeResult {
  ok: boolean;
  error?: string;
  /** Files written (for logging / debugging). */
  files?: string[];
}
```

### 3. Capability tiers — the mental model

```
┌────────────────────────────────────────────────────────────┐
│ FLEET TIER (Zund-owned, always bridged by every runtime)   │
│   memory · artifacts · docs · fleet-status · task-delegate │
│   ↑ declared via `fleet_capabilities:` in fleet YAML       │
└────────────────────────────────────────────────────────────┘
                        ↕ bridge (runtime-specific, runtime.bridgeFor)
┌────────────────────────────────────────────────────────────┐
│ RUNTIME TIER (runtime-internal, configured via pass-through)│
│   Pi: extensions + plugin kinds + builtin tools             │
│   Hermes: toolsets + plugin kinds + native skills           │
│   OpenClaw: skills + MCP + terminal backends                │
│   ↑ declared via `runtime_config:` in fleet YAML            │
└────────────────────────────────────────────────────────────┘
                        ↕ mcporter sidecar (ADR 0029)
┌────────────────────────────────────────────────────────────┐
│ MCP TIER (runtime-agnostic, shared across fleet)            │
│   GitHub, Linear, Notion, Slack, Google Workspace, etc.     │
│   ↑ declared via `mcp_servers:` in fleet YAML (ADR 0029)    │
└────────────────────────────────────────────────────────────┘
```

The invariant that makes this coherent: **skill / tool / extension /
MCP server / pack / fleet_capability / runtime_config are distinct
concepts that do not overlap.**

- **Skill** = markdown document (SKILL.md) describing a workflow or
  pattern. Prose only. References tools by name. Loaded on-demand via
  `load_skill`.
- **Tool** = callable function with name + schema + handler. The
  atomic unit. The LLM only ever sees tools.
- **Extension** = code (TS for Pi) loaded by the runtime at startup
  that registers tools. In-process, low-latency. Best for fleet
  primitives and latency-critical small capabilities.
- **MCP server** = external process exposing tools via MCP protocol
  (stdio/HTTP). Fetched on-demand via npx/uvx. Best for third-party
  integrations.
- **Pack** (ADR 0030) = opt-in bundle of skills + MCP servers +
  secrets-required. A distribution unit, not a runtime concept.
- **`fleet_capability`** = a Zund-owned primitive bridged into the
  runtime via `bridgeFor()`.
- **`runtime_config`** = opaque-to-Zund configuration injected into
  the runtime's native config file.

### 4. Secret binding during config injection

The `runtime_config:` block may reference secrets by the standard
`ref://secrets.X` syntax:

```yaml
runtime_config:
  web_backends:
    tavily:
      api_key: ref://secrets.TAVILY_API_KEY
    exa:
      api_key: ref://secrets.EXA_API_KEY
```

Resolution runs at `zund apply` time, **before** the rendered config
is written into the container. The runtime never sees the ref
syntax; it sees the plaintext secret as if it had been configured
natively.

Failed secret resolution **blocks the apply** and surfaces as a
`pending` state per ADR 0023: `pendingReason: "secret X required by
runtime_config not found"`, `suggestion: "zund secrets set X <value>"`.

Only the `runtime_config:` tree is scanned for refs. Other fields
(names, paths, models) are treated as literals.

### 5. Validation at apply time

When an agent's `fleet_capabilities:` lists a capability, the runtime
plugin's `bridgeFor(capability)` must return `ok: true`. If it does
not, apply fails with a clear error identifying:

- Which agent
- Which capability
- Which runtime
- The bridge error text

Example error:

```
apply failed: agent 'researcher' runtime 'hermes' capability 'mcp':
  bridgeFor returned: no MCP client available in Hermes 0.10.2 —
  upgrade to 0.11+ or remove 'mcp' from fleet_capabilities.
```

Responsibility: the runtime author ships a functioning
`bridgeFor()` for every capability they claim to support. Gaps surface
at apply time, not at first-tool-call time.

### 6. Three runtimes side by side

Same `fleet_capabilities:`, different `runtime_config:` — the seam
in action.

**Pi:**

```yaml
name: researcher
runtime: pi
fleet_capabilities: [memory, docs, fleet-status, task-delegate, mcp]
runtime_config:
  model: claude-sonnet-4
  extensions:
    web-search: { backend: tavily, api_key: ref://secrets.TAVILY_API_KEY }
    web-fetch: enabled
  builtin_tools: [bash, read, edit, write]
```

**Hermes:**

```yaml
name: researcher
runtime: hermes
fleet_capabilities: [memory, docs, fleet-status, task-delegate, mcp]
runtime_config:
  model: claude-sonnet-4
  terminal:
    backend: local
  web:
    default_backend: tavily
    backends:
      tavily: { api_key: ref://secrets.TAVILY_API_KEY }
      exa:    { api_key: ref://secrets.EXA_API_KEY, compress_with: gemini-flash }
  toolsets:
    - name: research
      tools: [search_web, fetch_url, summarize]
```

**OpenClaw:**

```yaml
name: researcher
runtime: openclaw
fleet_capabilities: [memory, docs, fleet-status, task-delegate, mcp]
runtime_config:
  model: claude-sonnet-4
  skills_registry: ~/.openclaw/skills
  terminal_backend: local
  mcp_servers_inline:   # openclaw-native mcp list
    - name: custom-internal
      command: uvx run my-internal-mcp
```

The Zund-facing surface (`fleet_capabilities:` + top-level agent
metadata) is identical; everything below it is the runtime's own
shape.

### 7. Relationship to ADR 0013's `tools:` field

ADR 0013 introduced `tools:` on agent YAML as a Pi-specific toggle
for built-in tool categories. That field stays, but is now scoped
inside `runtime_config:` rather than being a top-level agent field:

```yaml
# Before (ADR 0013):
tools: [bash, read, edit, write]

# After (this ADR):
runtime_config:
  builtin_tools: [bash, read, edit, write]
```

This is a narrow supersession. The `tools:` top-level field is
deprecated for v1.1 and removed in v2.0. For v1.0, the parser accepts
both and emits a deprecation notice. `fleet_capabilities:` is the new
top-level cross-runtime concept; runtime-specific toggles live under
`runtime_config:`.

## Challenges and open questions

### Config schema validation

Zund does not parse `runtime_config:`. The runtime plugin should —
each runtime plugin may declare a config schema (TypeBox or Zod) that
the daemon invokes at apply time for a friendly error. If the plugin
declines to validate, Zund passes the config through and any error
surfaces at container boot instead. Runtime authors are encouraged to
validate; not required.

### Config leakage into logs

`runtime_config:` may contain post-resolution plaintext secrets. The
daemon must not log the rendered config. Existing logging-discipline
test (ADR 0026 measure 6) extends to cover `runtime_config` render
paths.

### Versioning the runtime's native config

If Hermes ships a breaking change to `~/.hermes/config.yaml`, every
fleet YAML that declares `runtime: hermes` needs updating. Zund has
no way to migrate this for the user. Acceptable: this is identical
to the situation without Zund, and the runtime author controls the
cadence.

### Bridge failure isolation

If `bridgeFor("docs")` succeeds but the agent's first `docs_search`
call fails at runtime (e.g., the docs plugin crashed mid-session),
that is a service-plugin concern, not a bridge concern. The bridge is
responsible for wiring; the service plugin is responsible for staying
up. Observability for bridged-capability failures threads through the
existing `data-z:agent:error` event.

### What happens when `fleet_capabilities:` grows

The v1 set is six entries. Future entries (streaming-media bridges
per ADR 0031, federated memory, etc.) require an ADR and a matching
`bridgeFor()` in every runtime that claims to support them. A runtime
that doesn't support a capability must return `ok: false` with a
clear reason — silent no-op is not permitted.

## Consequences

**Makes easier:**

- **Multi-runtime fleets become tractable.** The same agent can run on
  Pi, Hermes, or OpenClaw with the fleet-YAML difference confined to
  `runtime_config:` — the Zund-facing surface is identical.
- **Runtime authors keep their shape.** No lossy abstraction over
  Hermes's web_tools.py or OpenClaw's skill registry. Drop it in
  `runtime_config:` and go.
- **Clean home for fleet primitives.** `fleet_capabilities:` is the
  one place to declare "this agent uses the fleet memory / queue /
  docs." The runtime plugin does the wiring.
- **Third tier falls out naturally.** MCP is just
  `fleet_capabilities: [mcp]` plus `mcp_servers:` (ADR 0029). Skills +
  MCP + secrets bundle via packs (ADR 0030).
- **Secret injection is uniform.** `ref://secrets.X` works inside
  `runtime_config:` the same way it works in `mcp_servers:` and
  pack manifests.

**Makes harder:**

- **Every runtime plugin must implement `bridgeFor()` for every
  capability it claims to support.** Surface grows with capability
  vocabulary. Mitigated by the ADR-amendment gate on new entries.
- **Fleet YAML gets bigger.** Pi agents that previously had a dozen
  lines now have `runtime_config:` with extension toggles and
  `fleet_capabilities:` with bridge names. Offset by clarity:
  reading the YAML tells you what the agent has.
- **Validation scatter.** Runtime plugins validate `runtime_config:`;
  Zund validates `fleet_capabilities:`; the secrets plugin validates
  refs. Three validators, not one. Mitigated by structured errors
  per ADR 0026's format.
- **Supersession of ADR 0013's `tools:` field.** Two-version transition
  needed.

## Relationship to existing ADRs

| ADR | Relationship |
|-----|-------------|
| 0003 | Extends `Runtime` with `configPath`, `configFormat`, `bridgeFor()`. |
| 0013 | The top-level `tools:` field is narrowly superseded; it moves under `runtime_config.builtin_tools`. Two-version migration. |
| 0020 | This ADR extends the runtime-tier plugin contract. Service-tier unchanged. |
| 0027 | Pi's concrete `bridgeFor()` implementation. The six fleet-tier extensions in that ADR map 1:1 onto this ADR's `fleet_capabilities:` entries. |
| 0029 | `fleet_capabilities: [mcp]` is what binds the mcporter sidecar into a given agent. |
| 0030 | Pack manifests declare which `fleet_capabilities:` they require (e.g., `mcp`) and feed additional MCP config into the sidecar. |
| 0023 | `task-delegate` capability is bridged into `POST /v1/tasks`. |
| 0025 | `docs` capability is bridged onto the active `DocsStore` plugin. |

## Implementation notes

**New contract additions:**

```
packages/core/src/contracts/runtime.ts     ← add RuntimePlugin interface
packages/core/src/contracts/capabilities.ts ← new: FleetCapability enum,
                                               BridgeContext, BridgeResult
```

**Fleet parser changes:**

```
packages/daemon/src/fleet/parser.ts
  - accept top-level runtime_config: (opaque YAML subtree)
  - accept top-level fleet_capabilities: (enum list)
  - deprecate top-level tools: with warning
packages/daemon/src/fleet/types.ts
  - AgentResource gains runtime_config?: unknown
  - AgentResource gains fleet_capabilities?: FleetCapability[]
```

**Daemon apply path:**

1. Parse fleet YAML.
2. For each agent: resolve `ref://secrets.*` in `runtime_config:`.
3. Load runtime plugin, serialize `runtime_config:` to the plugin's
   `configFormat`, write to `configPath` inside the container.
4. For each entry in `fleet_capabilities:`, call
   `runtime.bridgeFor(cap, ctx)`. Collect results.
5. If any bridge fails, mark agent as pending with reason; otherwise
   proceed to container start.

**Runtime plugin responsibilities (per-runtime):**

- Declare `configPath`, `configFormat`.
- Implement `bridgeFor()` for every capability the plugin advertises.
- Optionally: validate `runtime_config:` against a schema at apply
  time.

**No changes to:** service-tier plugins, the wire protocol (ADR 0022),
the task queue (ADR 0023), the docs store (ADR 0025).

## Open questions

- **Default `fleet_capabilities:` when the key is absent.** Probably
  `[memory, artifacts, docs, fleet-status]` — the fleet primitives
  every persistent agent needs. `task-delegate` and `mcp` stay opt-in
  to keep blast radius small.
- **Per-runtime capability support matrix.** A table in
  `docs/reference/runtimes.md` listing which runtime supports which
  capability — derivable from plugin manifests once runtimes
  implement `bridgeFor()`.
- **How runtime plugins express capability-schema requirements.** A
  capability may need more than its name — `task-delegate` needs the
  daemon endpoint; `mcp` needs the sidecar address. The `BridgeContext`
  shape covers v1, but may grow.
