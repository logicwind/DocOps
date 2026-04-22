# Zund Architecture

A one-page map of the system. Companion docs:

- **`reference/diagrams.md`** — mermaid diagrams (system architecture +
  fleet anatomy). Read first if you want the picture.
- **`reference/mental-model.md`** — vocabulary and concept relationships
  (skill vs tool vs extension vs MCP vs pack vs capability tier). This
  architecture doc is the *layer view*; the mental model doc is the
  *vocabulary view*. Read both.
- **`reference/decisions/`** — per-decision rationale (ADRs).
- **`reference/daemon.md`** — daemon internals.
- **`reference/runtime-protocol.md`** — wire format.

---

## The four layers

```
┌──────────────────────────────────────────────────────────────────────┐
│  L4  ACCESS                                                          │
│       CLI · Console · REST API · SSE (zund://stream/v1)              │  changes with UX
├──────────────────────────────────────────────────────────────────────┤
│  L3  ORCHESTRATION                                                   │
│       Dispatcher · Triggers · Runtime Registry · Event Translator    │  changes with product
├──────────────────────────────────────────────────────────────────────┤
│  L2  STATE                                                           │
│  ┌───────────┬───────────┬────────────────┬────────────────────────┐ │
│  │ Secrets   │ Memory    │ Artifacts      │ Sessions               │ │
│  │ age+sops  │ sqlite    │ local CAS      │ per-runtime JSONL idx  │ │
│  └───────────┴───────────┴────────────────┴────────────────────────┘ │  changes with schemas
├──────────────────────────────────────────────────────────────────────┤
│  Agent Runtime Interface   ┌──── pi ────┬──── vm ────┬──── ssh ────┐ │
│                            │ (current)  │ (future)   │ (future)    │ │
├──────────────────────────────────────────────────────────────────────┤
│  L1  SUBSTRATE                                                       │
│       Fleet Reconciler · Incus Client · Container Lifecycle         │  stable
│       Networking · Device Mounts · Image Management                  │
└──────────────────────────────────────────────────────────────────────┘
```

Each layer has one reason to change. The Runtime interface is the contract
that lets L1 host any agent runtime and L3 treat them uniformly. See
ADR 0001 for the full argument, ADR 0003 for the Runtime interface, ADR 0015
for L2 pluggability.

---

## L1 — Substrate

What makes containers run. Dumb, stable, boring on purpose.

| Component | Package path | Responsibility |
|-----------|--------------|----------------|
| Incus client | `apps/daemon/src/incus/client.ts` | HTTP over Unix socket (Bun native fetch) |
| Containers | `apps/daemon/src/incus/containers.ts` | Create, exec, list, delete |
| Devices | `apps/daemon/src/incus/devices.ts` | Mounts with `shift=true` |
| Host detection | `apps/daemon/src/incus/host.ts` | Reachable hostname for container-to-daemon |
| Image | `apps/daemon/src/incus/image.ts` | Pre-built `zund/base` |
| Fleet reconciler | `apps/daemon/src/fleet/` (parser, differ, validator) | YAML → plan → container CRUD |

**Key decisions:** Incus (ADR 0004), Bun native fetch (ADR 0006), pre-built
image (ADR 0007).

---

## Agent Runtime Interface

The contract between L1 (substrate) and L3 (orchestration). Each runtime
implementation knows how to launch an agent process, talk to it, stream its
events, and stop it.

```
interface Runtime {
  launch(agent: AgentResource, ctx: RuntimeContext): Promise<RuntimeSession>;
  session(agentName: string): RuntimeSession | null;
  events(): AsyncIterable<RuntimeEvent>;   // native events, pre-translation
  stop(agentName: string): Promise<void>;
  sessionStore: SessionStore;              // runtime owns its session shape
}
```

Today only `pi` is implemented (`apps/daemon/src/pi/`, will move to
`agents/runtimes/pi.ts`). VM and SSH are planned additions. See ADR 0003.

---

## L2 — State

What the system remembers. Four stores, each with a default implementation
and (eventually) a pluggable interface. See ADR 0015.

### SecretStore

- **Interface:** read-only from daemon perspective — decrypt and inject
- **Default (v0.3):** age + sops, encrypted `fleet/secrets/keys.yaml`
- **Pluggable:** yes, via `fleet/.sops.yaml` backend config (age → KMS → Vault)
- **Access:** daemon decrypts at apply time, injects as env vars into containers
- **ADR:** 0012

### MemoryStore

- **Interface:** `saveFact`, `searchFacts`, `listFacts`, `getWorkingMemory`, `setWorkingMemory`
- **Default:** `bun:sqlite`, two DBs — `sessions.db` (ephemeral Pi JSONL index, GC'd) + `memory.db` (permanent facts + working memory, FTS5)
- **Pluggable:** not yet — `MemoryDb` class uses SQLite directly
- **Access:** agents call Pi tools (`memory_save`, `memory_search`, `working_memory_update`)
- **ADR:** 0010 (current), 0016 (direction toward interface)

### ArtifactStore

- **Interface:** `ArtifactStore` (already defined)
- **Default:** `LocalArtifactStore` — content-addressed by SHA-256, blobs at `~/.zund/data/artifacts/blobs/<sha[0:2]>/<sha>`, metadata in `memory.db`
- **Pluggable:** yes (S3/MinIO drop in behind same interface)
- **Access:** agents call `zund_emit_artifact` Pi tool; daemon stores via interface; URL returned in tool_execution_end event
- **ADR:** 0011

### SessionStore

- **Interface:** owned by the Runtime (each runtime materializes sessions its own way)
- **Default (Pi runtime):** host-mounted directory per-agent with `shift=true` uid remap, Pi writes JSONL, daemon indexes in `sessions.db`
- **Pluggable:** follows the runtime — swap runtime, swap session shape
- **Access:** daemon reads via SessionStore; consumers see canonical session metadata regardless of runtime
- **ADR:** 0009 (current), 0003 (runtime interface ownership)

---

## L3 — Orchestration

What decides what runs and when. The "control plane."

| Component | Status | Responsibility |
|-----------|--------|----------------|
| Runtime Registry | planned | Map runtime name → implementation |
| Dispatcher | planned | LLM-backed routing of tasks → agents |
| Triggers | planned | Cron, webhook, event sources feeding the queue |
| Task Queue | planned | SQLite-backed durable work queue |
| Event Translator | planned | Native runtime events → zund://stream/v1 |
| Capability Index | planned | Derived from fleet YAML, rebuilt on apply |
| MCP Sidecar | planned | mcporter container per fleet, shared across agents (ADR 0029) |
| Pack Resolver | planned | Skill+MCP+secrets bundle resolution at apply time (ADR 0030) |

**Key decisions:** stream protocol (ADR 0022, supersedes 0002), runtime interface (ADR 0003), MCP sidecar (ADR 0029), packs (ADR 0030).

Implementation order and priorities live in `roadmap/next.md`.

---

## Capability model (orthogonal to L1–L4)

Agent capabilities are organized into three tiers that cut across the layer
stack. This is the vocabulary view of what Zund exposes to agents; the
mental model doc (`reference/mental-model.md`) is authoritative.

```
┌────────────────────────────────────────────────────────────────┐
│ FLEET TIER  (L2+L3, zund-owned, every runtime bridges to it)   │
│   memory · artifacts · docs · fleet-status · task-delegate     │
│   MCP-via-sidecar                                              │
└────────────────────────────────────────────────────────────────┘
                  ↕ bridge (runtime-specific mechanism)
┌────────────────────────────────────────────────────────────────┐
│ RUNTIME TIER  (runtime-internal; configured via runtime_config)│
│   Pi:     extensions + plugin kinds + builtin tools            │
│   Hermes: toolsets + plugins + native skills                   │
│   OpenClaw: skills + MCP + terminal backends                   │
└────────────────────────────────────────────────────────────────┘
                  ↕ mcporter sidecar (ADR 0029)
┌────────────────────────────────────────────────────────────────┐
│ MCP TIER  (runtime-agnostic, shared across fleet)              │
│   GitHub · Linear · Notion · Slack · GWS · Playwright · ...    │
└────────────────────────────────────────────────────────────────┘
```

- **Fleet tier** = capabilities that depend on cross-agent state. The
  Zund contract. Every runtime must bridge to each entry listed in an
  agent's `fleet_capabilities:` block. Pi bridges via extension
  generators; Hermes via its plugin system; OpenClaw via its skill
  architecture. **See ADRs 0027 (Pi extensions) and 0028 (fleet
  capabilities contract + runtime config pass-through).**
- **Runtime tier** = whatever the runtime ships natively. Zund does not
  abstract this. Configured via the fleet YAML `runtime_config:` block
  which is written verbatim into the runtime's native config file at
  container boot (with secret refs resolved). **See ADR 0028.**
- **MCP tier** = external processes exposing tools over MCP. One
  mcporter sidecar per fleet, lazy-launches servers via `npx -y <pkg>`,
  pools connections, caches OAuth tokens. Agents connect via loopback.
  **See ADR 0029.**

Capability distribution in v1:

| Capability | Tier | Where |
|---|---|---|
| `memory`, `artifacts`, `docs`, `fleet-status` | Fleet | Pi extension bridges today; see ADRs 0010/0011/0025 |
| `task-delegate` | Fleet | New Pi extension per ADR 0027; wraps `POST /v1/tasks` |
| `web-search`, `web-fetch` | Runtime (Pi) | Pi extensions per ADR 0027 |
| `browser` | MCP | `playwright-mcp` via sidecar |
| `github`, `linear`, `notion`, `slack`, `gws` | MCP | Community/official MCP servers via sidecar |
| `terminal`, `file-ops`, `shell` | Runtime | Pi builtin; Hermes builtin; etc. |

Packs (ADR 0030) are the distribution unit that bundles skill + MCP +
secrets. On `zund apply`, packs resolve into: skill files copied into the
agent workspace, MCP configs unioned into the sidecar, secrets validated.

---

## L4 — Access

How the world talks to Zund. Uniform wire format below the UX.

| Surface | Transport | Protocol |
|---------|-----------|----------|
| CLI | Unix socket (`zundd.sock`) | REST + SSE |
| Console | TCP `:4000` | REST + SSE |
| External / webhooks | TCP `:4000` | REST |

All streaming uses SSE over a single canonical vocabulary (`zund://stream/v1`).
See `reference/runtime-protocol.md` and ADR 0002.

**Key decisions:** dual Bun.serve (ADR 0008), message endpoint (ADR 0014),
stream protocol (ADR 0002).

---

## How a request flows

Three representative flows:

### Fleet apply (static config → running agents)

```
CLI/Console  ──POST /v1/apply──▶  L4
                                    │
                                    ▼
                                  L1 Fleet Reconciler
                                    │  diff → plan
                                    ▼
                                  L1 Incus Client  ──create/delete containers──▶
                                    │
                                    ▼
                                  Runtime.launch()  ──start agent process──▶
                                    │
                                    ▼
                                  L2 SessionStore  (track session state)
```

### Message an agent (user → agent → response)

```
CLI/Console  ──POST /v1/agents/:name/message──▶  L4
                                                   │
                                                   ▼
                                                 Runtime.session(name)
                                                   │
                                                   ▼
                                                 Runtime emits native events
                                                   │
                                                   ▼
                                                 L3 Event Translator  ──zund://stream/v1──▶  L4 SSE
```

### Task dispatch (planned, AI-first flow)

```
cron / webhook / API  ──enqueue──▶  L3 Task Queue (L2-backed)
                                       │
                                       ▼
                                     L3 Dispatcher (LLM-routed)
                                       │
                          ┌────── match? ──────┐
                          ▼                    ▼
                 Runtime.launch()          Pending (+reason)
                 (ephemeral agent)
                          │
                          ▼
                 Agent does work
                          │
                          ▼
                 POST /v1/tasks/:id/result
                          │
                          ▼
                 L2 ArtifactStore + result event → L4 SSE
```

---

## The OSS / commercial boundary

The four-layer model aligns with the license split. See ADR 0001 for the
full argument.

| Layer | OSS | Commercial |
|-------|-----|------------|
| L1 Substrate | ✓ always | — |
| L2 State (single-tenant defaults) | ✓ | — |
| L2 State (federated, hosted) | — | ✓ |
| L3 Orchestration (local dispatcher) | ✓ | — |
| L3 Orchestration (marketplace, learned policies) | — | ✓ |
| L4 Access (CLI, daemon API) | ✓ | — |
| L4 Access (hosted console, mobile) | — | ✓ |

---

## Component map (code reality)

Where things live today. Update this table when the shape changes.

```
apps/daemon/src/
├── incus/          L1 Substrate — Incus client, containers, devices, image
├── fleet/          L1 Substrate — YAML parser, differ, validator, reconciler
├── pi/             Runtime (current) — agent launcher, RPC (moves to agents/runtimes/pi/)
├── secrets/        L2 State — SecretStore (age+sops)
├── memory/         L2 State — MemoryStore (sqlite + FTS5)
├── artifacts/      L2 State — ArtifactStore (local CAS, pluggable interface)
├── sessions/       L2 State — SessionStore (Pi JSONL index)
├── skills/         Cross-layer — skill provisioning
└── api/            L4 Access — HTTP router, routes per resource

apps/cli/       L4 Access — thin HTTP wrapper, zero business logic
apps/console/   L4 Access — web UI
```

Planned additions under `apps/daemon/src/`:
- `agents/` — `runtime.ts` (interface), `registry.ts`, `runtimes/pi/`
- `queue/` — task queue (SQLite, ADR 0023)
- `dispatcher/` — LLM routing (ADR 0023)
- `triggers/` — cron, webhook, event sources (ADR 0023)
- `capability/` — capability index + `fleet_capabilities` contract (ADR 0028)
- `stream/` — protocol v1 vocabulary + translator (ADR 0022)
- `mcp/` — mcporter sidecar lifecycle, per-fleet container management (ADR 0029)
- `packs/` — pack resolver, skill copier, MCP config unioner (ADR 0030)

Planned additions under `packages/`:
- `packages/packs/` — bundled pack definitions (research-primitives, github-workflow, productivity-gws, team-ops, docs-io, browser-automation per ADR 0030)
- `packages/plugins/web-search-tavily/` — default web-search backend (ADR 0027)
- `packages/plugins/web-search-brave/` — alternative web-search backend
- `packages/plugins/runtime-pi/src/extensions/` — the seven extension generators (ADR 0027)
- `packages/plugins/media-*` — STT/TTS/Vision/ImageGen plugin kinds (ADR 0031, future)

---

## How to extend this doc

- **Architecture doesn't decide; ADRs do.** If you find yourself writing
  "we chose X because Y" here, it belongs in `reference/decisions/`.
- **Keep it a map, not a spec.** Pointers to code + pointers to decisions.
  The code is the spec; this doc helps you find the right piece fast.
- **Update the component map when packages move.** Drift here misleads
  every future reader.
