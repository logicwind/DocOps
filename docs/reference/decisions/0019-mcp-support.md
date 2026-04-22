---
id: "0019"
title: "MCP (Model Context Protocol) support — required; architecture open"
date: 2026-04-16
status: draft
implementation: not-started
supersedes: []
superseded_by: null
related: ["0003", "0007", "0013", "0018"]
tags: [mcp, tools, extension, pi, l2, l3]
---

# 0019 · MCP (Model Context Protocol) support — required; architecture open

Date: 2026-04-16
Status: draft (decision accepted, architecture open)
Evidence: `experiments/12-pi-capabilities-audit/pi-capabilities.md` §3
Related: ADR 0003 (runtime interface), ADR 0007 (prebuilt image), ADR 0013
(Pi tools field), ADR 0018 (agent lifecycle)

## Context

MCP is the emerging standard for tool/resource connections between agents
and external services. As of 2026-04 there are ~1000+ MCP servers
available (databases, cloud providers, SaaS, internal tools). For Zund to
credibly compete with OpenClaw/Hermes/Claude Code on tool breadth, MCP
support is not optional.

**Current runtime state:**
- **Pi (0.65.2):** No native MCP support. Audit confirms no MCP client,
  no `@modelcontextprotocol/sdk` dependency. Custom tools must be bridged
  via Pi's `registerTool()` extension API.
- **Hermes:** Native MCP support (per its README — "Connect any MCP
  server for extended capabilities").
- **OpenClaw:** Status unknown (not yet audited); runs on the same Pi
  core so likely same gap unless OpenClaw ships its own bridge.

Any Zund MCP architecture must work for at least Pi today, and must not
lock us out of using native-MCP runtimes (Hermes, future OpenClaw) when
they're adopted under ADR 0018 Phase 2.

Additionally:
- MCP servers come in two transport flavors: **stdio** (subprocess) and
  **HTTP/SSE** (network).
- Some MCP servers are local-only (stdio wrapping a CLI); some are remote
  services (HTTP); many require auth (API keys, OAuth).
- Secrets must flow from Zund's secret store (ADR 0012) into MCP
  connections without ending up in agent YAML or container images.
- MCP server lifecycle (start, health, reconnect, scale) is a substrate
  concern. Pi should not manage it.

## Decision

**Accepted:** Zund will support MCP as a first-class tool extension
mechanism. Fleet YAML will declare MCP servers; agents reference them
declaratively; tools appear in the agent's active toolset.

**Open (to decide before implementation):** *how* the MCP-to-runtime
bridge is structured. Four candidate architectures below; recommendation
at the end.

### Candidate architectures

#### Option A — Pi extension per MCP server (direct bridge)

Each MCP server declared in fleet YAML translates into a Pi extension
loaded inside the container. The extension owns the MCP client, connects
to the server, registers tools via `pi.registerTool()`.

```
Agent container
  └── Pi
      ├── extension: mcp-bridge-cloudflare.ts
      │   └── MCP client → https://mcp.cloudflare.com (HTTP/SSE)
      ├── extension: mcp-bridge-tally.ts
      │   └── MCP client → stdio: mcp-tally subprocess
      └── extension: mcp-bridge-github.ts
          └── MCP client → https://...
```

**Pros:**
- Shortest path to working. No new daemon services.
- Uses existing Pi extension loader (already in use for Zund's built-in
  tools).
- One file per MCP server, easy to generate from YAML.

**Cons:**
- Every agent container runs its own MCP clients → N agents × M servers
  connections.
- MCP server lifecycle happens inside the agent container (for stdio
  transports) — violates L1/L2/L3 separation.
- Tight coupling to Pi's unstable extension API (audit flags API
  stability as "low to medium"; breaks expected per Pi minor version).
- When we adopt Hermes/OpenClaw as runtimes, their native MCP doesn't
  reuse this bridge — duplicated configuration surface.

#### Option B — Daemon-spawned sidecars + per-runtime thin bridge

Zund daemon owns MCP server lifecycle. Each MCP server declared in fleet
YAML becomes a daemon-managed process (or sidecar container). Agent
containers connect to these via a stable internal endpoint. The runtime
(Pi, Hermes, etc.) uses a thin bridge extension to register the sidecars'
tools.

```
Zund daemon
  ├── MCP sidecar: cloudflare-mcp (managed)
  ├── MCP sidecar: tally-mcp (managed)
  └── MCP sidecar: github-mcp (managed)
           │
           │  HTTP endpoints on daemon-internal network
           ▼
Agent container
  └── Pi
      └── extension: zund-mcp-bridge.ts
          └── Connects to daemon-managed sidecars
              (discovers them via daemon API)
```

**Pros:**
- Lifecycle (start, health, restart, auth injection) lives in the
  daemon — correct architectural layer.
- Secrets injected into sidecars once, not into every agent container.
- Sidecars shared across multiple agents — N connections, not N × M.
- Thin Pi bridge is runtime-agnostic skeleton; Hermes/OpenClaw bridges
  become small adapters to the same sidecar set.
- Sidecars can be restarted, versioned, and observed uniformly.

**Cons:**
- More moving parts: daemon gains MCP server lifecycle management.
- Requires a small runtime-to-daemon API for sidecar discovery (extends
  the existing zundd API).
- Still requires a Pi bridge extension (though a thinner, simpler one).
- Remote-only MCP servers (HTTPS) don't need sidecars — need to
  differentiate "managed by daemon" vs "called directly."

#### Option C — Zund as MCP host (daemon exposes a unified MCP endpoint)

Zund daemon speaks MCP *outward* to all configured MCP servers AND
exposes its own aggregated MCP endpoint *inward* to the agent. The
runtime connects to a single MCP endpoint (the daemon's) and sees all
tools as one unified surface.

```
MCP server: cloudflare ─┐
MCP server: tally ──────┼──► Zund daemon (MCP host)  ──► exposes unified MCP endpoint
MCP server: github ─────┘                                   │
                                                            ▼
                                                 Agent container
                                                   └── Pi extension:
                                                       connects to ONE MCP endpoint
                                                       (zund daemon internal socket)
```

**Pros:**
- Single connection from each agent. Aggregation, auth, routing, audit
  all in one place.
- Most architecturally clean — Zund becomes part of the MCP ecosystem
  (can itself be an MCP server consumed by external clients).
- Perfect fit for shared-state philosophy: all tool calls pass through
  the daemon, fleet-level audit stream trivially captures them.
- Future runtimes with native MCP (Hermes) drop in with zero custom
  bridge — they just connect to Zund's MCP endpoint.

**Cons:**
- Daemon becomes a stateful MCP host — non-trivial implementation
  (protocol translation, session management, concurrent requests).
- Still needs Pi bridge (one extension) because Pi doesn't speak MCP.
  Bridge is minimal (one endpoint) but required.
- Implementing MCP host is more work than using an MCP client library.

#### Option D — Runtime-owned MCP (no Zund abstraction)

Each runtime handles MCP its own way. Fleet YAML declares `mcp:` config;
the Runtime interface (ADR 0003) passes it through; each runtime is
responsible for connecting.

**Pros:**
- Minimal abstraction; uses each runtime's natural path.
- No daemon-level MCP code at all.

**Cons:**
- Pi still needs a bridge built by us (Option A, essentially).
- No unified observability across runtimes — audit stream fragments.
- Fleet-level features (cost tracking per tool, rate limiting, shared
  auth) become impossible.
- Per-runtime configuration drift; user surface inconsistent.

### Recommendation

**Phase 1: Option B (daemon-spawned sidecars + thin Pi bridge).**
**Phase 2: migrate toward Option C if demand warrants it.**

Rationale:
- **Option A is simplest but wrong-layered.** Putting MCP lifecycle in
  every agent container violates Zund's substrate/runtime separation and
  doesn't scale when we adopt more runtimes.
- **Option C is ideal but expensive.** Implementing a production MCP
  host is a real undertaking. Worth doing only once there's clear
  demand.
- **Option B is the pragmatic middle.** Daemon owns lifecycle (correct
  layer), sidecars shared across agents (scales), thin bridge per
  runtime (minimal Pi coupling). It's a stepping stone to Option C —
  Option C is Option B with the daemon itself becoming MCP-protocol
  aware.
- **Option D is a non-starter** because it gives up the fleet-level
  observability thesis (see braindump: fleet-level features as USP).

## What fleet YAML looks like

```yaml
# fleet/mcp/cloudflare.yaml
kind: MCPServer
name: cloudflare
transport: http
url: "https://mcp.cloudflare.com"
auth:
  type: bearer
  secret: CLOUDFLARE_API_TOKEN   # resolved from fleet secrets (ADR 0012)
```

```yaml
# fleet/mcp/tally.yaml
kind: MCPServer
name: tally
transport: stdio
command: "mcp-tally"
args: ["--read-only"]
env:
  TALLY_DB_PATH: /mnt/tally.duckdb
scope: shared                    # single sidecar shared across agents
```

```yaml
# fleet/roles/researcher.yaml
kind: Role
name: researcher
image: zund/researcher:v3
mcp: [cloudflare, tally]         # reference by name; tools auto-registered
```

Under Option B, daemon reconciles `fleet/mcp/*.yaml`:
- Remote (HTTP) servers: no sidecar; track as "external."
- Local (stdio) servers: spawn a daemon-managed sidecar process.
- On agent launch, bridge extension queries daemon API for the agent's
  configured MCP servers and registers their tools into Pi.

## Challenges and open questions

- **Per-agent vs shared sidecar.** `scope: shared` vs `scope: per-agent`
  in YAML. Shared sidecars scale better but share state — not safe for
  tenant-isolated setups. Default to shared for OSS, allow per-agent.
- **Secret injection into sidecars.** Must use the same ADR 0012 flow
  as agent containers. Sidecars run outside agent containers, so secret
  mount points differ.
- **Bridge extension stability.** Pi's extension API is unstable (ADR
  audit). Bridge must be version-pinned against Pi and tested on
  upgrade. Goal: keep bridge <200 lines so updates are cheap.
- **Discovery protocol between bridge and daemon.** New small HTTP
  endpoint (`GET /v1/agents/:name/mcp-servers`) that returns
  `{ name, endpoint, schema }` list. Bridge calls once at startup.
- **Reconnect on sidecar restart.** Daemon emits event; bridge
  re-registers tools. Needs thought — Pi doesn't have tool
  *unregister*, only `setActiveTools`.
- **Tool name collisions.** Two MCP servers expose `create_issue` — Pi
  requires unique tool names. Namespace them: `cloudflare.create_worker`
  vs `github.create_issue`.
- **Cost/quota tracking.** Phase 2 / Pro-tier feature — daemon
  observes all MCP calls, meters per agent/role.
- **Audit stream.** Every MCP call should emit a zund://stream/v1 event
  (ADR 0002). Under Option B the bridge emits; under Option C the
  daemon emits directly.
- **Fallback when MCP server unreachable.** Agent gets a clean tool
  error, not a hang. Needs timeout discipline in the bridge.

## Interaction with ADR 0018 (Open/Pinned/Spot lifecycle)

- MCP config lives in YAML → changes are declarative → reconciled on
  `zund apply`. Same path as skills/packages.
- In **Open** mode, an agent may discover a useful MCP server and
  propose adding it. Phase 2 proposal stream (ADR 0018) extends to
  `MCPProposal { name, url, scope }`. Approval → write
  `fleet/mcp/<name>.yaml` → apply.
- **Pinned/Spot** agents only get MCP servers declared in YAML. No
  runtime discovery in those tiers.
- Changing a Pinned agent's `mcp:` list triggers a rolling restart
  (same flow as image bump).

## Interaction with ADR 0003 (Runtime interface)

- The Runtime interface gains an optional `mcpClient(serverName) →
  MCPConnection` method (or similar).
- Pi runtime implements it via the bridge extension.
- Hermes runtime (when adopted) implements it by passing `mcp:` config
  to Hermes's native MCP subsystem.
- OpenClaw runtime (when adopted) implements it by whatever OpenClaw
  provides (TBD — requires separate audit).

## Consequences

**Makes easier:**

- Instant access to the ~1000 existing MCP servers without writing tool
  code per-service.
- Fleet-wide observability of external tool calls (Option B/C).
- Clean declarative extension model that scales with Zund's OSS/Pro
  seam (shared MCP infra is a natural Pro feature).
- Feature parity with Hermes/OpenClaw on tool breadth without building
  it ourselves.

**Makes harder:**

- Introduces daemon-level sidecar lifecycle management (Option B).
- Maintenance burden for Pi bridge extension, coupled to Pi's unstable
  API.
- Additional fleet YAML kind (`MCPServer`) and validator work.
- Secret-flow complexity (sidecars need the same secrets machinery
  agents have).

## Implementation notes

**Phase 1 deliverables (if Option B confirmed):**

1. `MCPServer` resource kind in `packages/daemon/src/fleet/parser.ts` +
   validator.
2. `packages/daemon/src/mcp/` — sidecar lifecycle manager (spawn, health,
   restart, secret injection).
3. Daemon HTTP endpoint: `GET /v1/agents/:name/mcp-servers` returning
   endpoint/schema list.
4. Pi bridge extension: `packages/daemon/src/pi/extensions/mcp-bridge.ts`
   (target <200 LOC). Loaded into agent containers on launch.
5. Role YAML gains `mcp: [names]` field.
6. Rolling-restart logic in executor when role's `mcp:` list changes.
7. Event stream: emit `mcp.tool_call.started/succeeded/failed` events.

**Out of scope for Phase 1:**

- Per-agent MCP (only `scope: shared` sidecars in MVP).
- Auto-discovery of tools (proposal stream for MCP — that's Phase 2 of
  ADR 0018).
- Daemon as full MCP host (Option C migration).
- OAuth flows for MCP servers (only bearer/header auth in MVP).
- UI for MCP server management in console (manual YAML edit in MVP).

## Next steps

- Review this ADR and lock the architecture choice (A/B/C/D). Until
  chosen, no implementation begins.
- If Option B accepted: spike the sidecar lifecycle on one real MCP
  server (e.g., the Cloudflare MCP) before generalizing.
- Audit OpenClaw for native MCP status — informs runtime-adapter design.
- Pin Pi version and add bridge-extension tests to the Pi upgrade gate.
