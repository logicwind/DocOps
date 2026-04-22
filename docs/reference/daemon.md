# zundd architecture

The zund daemon (`zundd`) manages **fleets** of AI agents running as processes
inside Incus system containers. This document describes the internal
pipelines, state model, and container layout so a new reader (human or
assistant) can locate any piece of behavior in one hop.

## Top-level picture

```
         ┌──────────────┐
         │  CLI / UI    │  HTTP + SSE
         └──────┬───────┘
                │
       ┌────────┴────────┐
       │     zundd       │  in-memory AppState
       │  (api/server)   │  fleetState: Map<key, Resource>
       │                 │  agents:     Map<name, AgentHandle>
       └──┬──────────┬───┘
          │          │
      Incus API    Pi RPC (unix socket per agent)
          │          │
          ▼          ▼
    ┌─────────┐  ┌──────────────────────┐
    │ zund-*  │  │  /root/.pi/agent     │  settings.json, auth.json
    │ cont-   │  │  /root/.pi/agent/ext │  zund-fleet extension
    │ ainer   │  │  /skills/<name>      │  read-only bind-mounts
    │         │  │  /workspace          │  RW bind-mount (host .zund/data)
    └─────────┘  └──────────────────────┘
```

One daemon. One Incus. N containers, one per agent. Each container runs a
single Pi process. The daemon talks to Incus over the unix
socket `/var/snap/incus/common/incus/unix.socket` and to Pi via per-agent RPC
pipes mediated by `pi/rpc.ts`.

## Source layout

```
apps/daemon/src/
├── api/server.ts        HTTP + SSE. Routes, apply handler, debug endpoint.
├── fleet/
│   ├── parser.ts        YAML → Resource[]  (file, dir, or inline string)
│   ├── validator.ts     cross-resource validation (dangling refs, etc.)
│   ├── defaults.ts      fills in optional fields
│   ├── differ.ts        diffFleet(desired, current) → DiffPlan
│   ├── executor.ts      walks the plan, calls launcher, handles errors
│   └── types.ts         Resource union, DiffPlan, ExecutorState
├── incus/
│   ├── client.ts        IncusClient (unix socket HTTP, wait-for-op)
│   ├── containers.ts    list/create/exec/config/stop/delete containers
│   └── devices.ts       mountHostDir (bind-mount a host path readonly)
├── pi/
│   ├── launcher.ts      createContainer → mount → writePiConfig → start RPC
│   ├── config.ts        writes /root/.pi/agent/{settings,auth,models}.json
│   ├── extension-writer.ts writes the zund-fleet Pi extension into the box
│   └── rpc.ts           AgentRpcSession — per-agent stdin/stdout pipe
├── skills/
│   ├── loader.ts        validates a SKILL.md directory (frontmatter etc.)
│   └── provisioner.ts   copy → ~/.zund/data/skills/<name>, mount via Incus
├── memory/
│   ├── db.ts           SQLite (Bun:sqlite), facts + FTS5 + working_memory
│   └── markdown.ts     pure ## H2 slot parser used by PATCH /working-memory
├── secrets/
│   ├── vault.ts         SOPS+age encrypt/decrypt; readAllSecrets() called at apply time
│   ├── resolver.ts      resolveFleetSecrets() — pure, synchronous, all-errors-collected
│   ├── age.ts           age key generation and path helpers
│   └── sops.ts          thin wrapper around the sops binary
├── sessions/
│   ├── indexer.ts       scans ~/.zund/data/sessions/<agent>/*.jsonl
│   └── gc.ts            deletes old session files
├── log.ts               structured logger (createLogger("api") etc.)
└── index.ts             main entry; starts socketServer + tcpServer
```

See [`docs/secret-management.md`](../../docs/secret-management.md) for the full secrets lifecycle, resource schema, resolution algorithm, and upgrade path.

## The apply pipeline

The single most important flow. Covers create, update, delete in one call.

```
POST /v1/apply { path | content | resources[], preview?, force? }
    │
    ▼
 api/server.ts  handleApply
    │  1. parse body   → Resource[] desired
    │                    Compute fleetDir (abs dir for file/dir input, else undefined)
    │  2. inline-skill guard: if fleetDir===undefined, relative skill
    │                          source.path is a 422 validation error
    │  3. applyDefaults(desired)
    │  4. validateFleet(desired)  → errors[], warnings[]
    │  5. diffFleet(desired, state.fleetState values) → DiffPlan
    │  6. if body.preview → return plan + errors + warnings (no execute)
    │  7. if errors.length > 0 → 422 with plan
    │  8. fleetName guard — cannot rename running fleet
    │  9. executeFleetPlan(plan, desired, state, { force, fleetDir })
    │ 10. on success: replace state.fleetState, set state.fleetName,
    │                 persistFleetState → ~/.zund/fleet-state.yaml
    │
    ▼
 fleet/executor.ts  executeFleetPlan
    │  creates: Promise.allSettled over agentCreates
    │     └─► resolveAgentSkills(agent, desired, fleetDir ?? cwd)
    │            │  throws on any missing/invalid skill → pushes errors[],
    │            │  returns before container create
    │            ▼
    │     └─► launchAgent(incusClient, config)
    │
    │  updates (sequential, destroy+recreate):
    │     └─► resolveAgentSkills BEFORE destroy (old agent survives if bad)
    │     └─► destroyAgent → launchAgent
    │
    │  destroys (parallel):
    │     └─► destroyAgent
    │
    ▼
 pi/launcher.ts  launchAgent
    │  1. mkdirSync sessionsDir, workspaceDir on host
    │  2. createFromBaseImage(zund/base)
    │  3. mountHostDir sessionsDir   → /root/.pi/agent/sessions
    │  4. mountHostDir workspaceDir  → /workspace
    │  5. for each skill: mountHostDir skill.hostPath → /skills/<name> (RO)
    │  6. setContainerConfig environment.<k>=<v>
    │  7. writePiConfig({ provider, model, apiKey, baseUrl, skillPaths })
    │     └── writes /root/.pi/agent/settings.json with "skills": [...]
    │         and "enableSkillCommands": true when skillPaths are present
    │  8. handle.rpcSession.start() → handle.status = "running"
    │
    ▼
 handle goes into state.agents, fleet state persisted
```

## State lifecycle

In-memory, held in `AppState` (see `api/server.ts:43`):

| Field            | Shape                                  | Source of truth                                                                                                            |
| ---------------- | -------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| `fleetState`     | `Map<"kind:name", Resource>`           | Persisted to `~/.zund/fleet-state.yaml` on every successful apply. Reloaded on daemon start via `loadPersistedFleetState`. |
| `agents`         | `Map<name, AgentHandle>`               | Ephemeral. Rebuilt by scanning Incus for `zund-*` containers on startup.                                                   |
| `fleetName`      | `string \| undefined`                  | From the `kind: fleet` resource in the current apply.                                                                      |
| `memoryDb`       | `MemoryDb`                             | `~/.zund/data/memory.db`                                                                                                   |
| `sessionIndexer` | `SessionIndexer`                       | `~/.zund/data/sessions.db` (scans `~/.zund/data/sessions/<agent>/*.jsonl`)                                                 |
| `sseClients`     | `Set<ReadableStreamDefaultController>` | Connected `/v1/events` subscribers.                                                                                        |

**Key invariant:** `fleetState` is the desired state. `agents` is the running
state. The differ compares these two to decide what to do next apply.

**Restart behavior:** on daemon restart, containers keep running (Incus owns
them). `fleetState` is rehydrated from YAML. The `agents` map is rebuilt by
opening RPC sessions to each existing `zund-*` container. Skills already
mounted stay mounted — no re-provisioning happens on restart.

## The skill pipeline

1. User declares `kind: skill` resources with `source: { type: "local", path: "./skills/x" }`.
2. At apply, `fleetDir` is the directory of the submitted YAML file (or undefined for inline).
3. `executor.resolveAgentSkills(agent, desired, fleetDir)` for each skill the agent uses:
   - finds the matching `SkillResource`
   - calls `skills/provisioner.ts::resolveLocalSkill(skill, fleetDir)`
     - resolves `source.path` against `fleetDir` if relative
     - `validateSkillDir` reads `SKILL.md` frontmatter (name, description, allowed-tools)
     - `cpSync` copies the dir to `~/.zund/data/skills/<name>/`
     - returns `{ name, hostPath }`
4. `launcher.launchAgent` mounts each skill to `/skills/<name>/` read-only in the container.
5. `pi/config.ts::writePiConfig` writes `"skills": ["/skills/haiku-writer", ...]` and
   `"enableSkillCommands": true` into `/root/.pi/agent/settings.json`. Without this, Pi
   never scans the mount points and the skills are invisible to the agent.

**Quirk — differ cascade is missing.** Changing a skill resource
(`source.path` or content) is detected as a `skill:*` update by `differ.ts`
but **does not** mark consumer agents as dirty. So an edit to a skill
referenced by a running agent will silently persist to `fleetState` with no
effect on the container. Workaround today: delete and reapply the fleet.
Future fix: cascade `skill` updates into `agent` updates in `differ.ts`.

## The memory system

SQLite at `~/.zund/data/memory.db` (see `memory/db.ts`).

Two tables:

- `facts(id, agent, content, scope, created_at)` + FTS5 index — discrete facts for recall
- `working_memory(agent, scope, content, updated_at)` — structured per-agent context docs, upsert by `(agent, scope)`

### Fact scopes

Namespaced strings: `agent:<name>`, `team:<name>`, `fleet:<name>`. No
cross-scope aggregation — callers pick. `searchFacts`/`listFacts` default to
`agent:{name}` when scope omitted, so forgetting a scope never leaks
team/fleet facts into a private query.

### Fact dedup + GC

`saveFact` is idempotent on `(agent, scope, content)` — re-saving the same
fact returns the existing row instead of creating a duplicate. Each `(agent,
scope)` bucket is also capped at `MAX_FACTS_PER_SCOPE = 500`; once the cap
is hit, the oldest row is evicted on the next save. Manual cleanup is exposed
via `pruneFacts({ agent, scope?, olderThanMs? })` and `countFacts(agent,
scope?)`. The console /memory route uses these to power "prune older than N
days" buttons.

### Working memory — three scopes

Inspired by Mastra's memory module: template-driven schema, system-prompt
injection layer, and read-only shared context.

| Scope  | Storage key               | Write path                                                 | Lifetime                                                    |
| ------ | ------------------------- | ---------------------------------------------------------- | ----------------------------------------------------------- |
| agent  | `(agentName, "agent")`    | `working_memory_update` tool (`scope: "agent"`, default)   | Persistent across sessions                                  |
| thread | `(agentName, "thread:{sessionId}")` | `working_memory_update` tool (`scope: "thread"`) | Single Pi session; GC'd on next `session_start`             |
| fleet  | `(fleetName, "fleet")`    | Seeded from `kind: fleet` `workingMemory` field on apply   | Replaced on re-apply; cleared if field removed. Read-only to agents. |

### HTTP endpoints

- `GET /v1/agents/:name/memory?q=&scope=` — search / list facts
- `POST /v1/agents/:name/memory { content, scope }` — save a fact (dedup on `(agent, scope, content)`)
- `DELETE /v1/agents/:name/memory?scope=&olderThanMs=` — bulk prune
- `DELETE /v1/agents/:name/memory/:id` — delete one fact by id
- `GET /v1/agents/:name/working-memory?scope=` — read (`agent` or `thread:{sessionId}`)
- `PUT /v1/agents/:name/working-memory { content, scope }` — full-document replace
- `PATCH /v1/agents/:name/working-memory { slot, operation, content?, scope }` — surgical ## H2 slot update (`replace` | `append` | `delete`); 404 if slot doesn't exist
- `GET /v1/agents/:name/working-memory/scopes` — enumerate scopes present
- `DELETE /v1/agents/:name/working-memory/gc { scopePrefix, exceptScope }` — GC stale thread entries
- `GET /v1/fleet/:name/working-memory` — read fleet shared context (read-only; no PUT)

### How agents use it

`pi/extension.ts::generateExtension` writes a self-contained Pi extension
into every container at launch. It registers six tools —
`zund_fleet_status`, `memory_save`, `memory_search`, `working_memory_update`,
`working_memory_patch`, `load_skill` — plus `before_agent_start` and
`session_start` event hooks. Tool calls and WM fetches both go through
`${ZUND_API_URL}`, which the executor injects at launch time.

**Per-turn injection (`before_agent_start`).** Fetches all three WMs in one
`Promise.all` and appends a three-section block to `systemPrompt`:

```
## Shared Context (read-only)        ← fleet WM, if non-empty
## Working Memory (persistent)       ← agent WM (or template fallback)
## Working Memory (this session)     ← thread WM, if non-empty
```

Injected into `systemPrompt`, **not** into the user message stream. Stable
bytes across turns → prompt-cache hit. This also prevents the model from
treating WM as enumerable structured data to recite back verbatim (the
original "list-of-scopes" verbose-recall bug).

**Template fallback.** Each `kind: agent` resource can carry a
`workingMemoryTemplate` markdown schema. When the DB has no saved agent WM,
the template is injected instead, giving the model a shape to fill. Flow:
`fleet.yaml → AgentResource → ExtensionConfig → const WORKING_MEMORY_TEMPLATE`
embedded in the generated extension.

**Session GC (`session_start`).** Every new / resumed / forked session fires
`DELETE /v1/agents/:name/working-memory/gc` with the current `sessionId` as
`exceptScope`. Stale `thread:*` entries for this agent are wiped; the current
session's thread WM survives.

**Tool scopes.**

- `working_memory_update` schema is `"agent" | "thread"`. No `"fleet"` literal — agents cannot write shared context.
- `working_memory_patch` is the cheap path: `{ slot, operation: replace|append|delete, content?, scope: agent|thread }`. It targets one `## H2` section by heading text (case-insensitive) using `memory/markdown.ts`, leaves other sections untouched, and 404s if the slot doesn't exist (use `working_memory_update` to add new sections). Concurrent slot edits no longer race, since each call rewrites only its own slot.
- `memory_search` defaults to `agent:{name}` and its result formatter drops scope labels and dedupes by content, so the model sees clean facts, not raw `[agent:name]` tags it can hallucinate into fake "scopes".

**Per-agent built-in tool selection.** `AgentResource.tools?: string[]`
plumbs through `ExtensionConfig` to a `pi.setActiveTools()` call at
`session_start`. `undefined` (default) inherits Pi's built-in defaults
(`read, bash, edit, write`); `[]` disables all built-ins (skill-only or
chat-only agents); a non-empty array is the exact allowlist. Zund's own
extension tools (`memory_*`, `working_memory_*`, `zund_fleet_status`,
`load_skill`) are always available regardless of this setting.

**Reach-back host name.** Production hardcodes
`ZUND_API_URL=http://host.incus:4000` (see `fleet/executor.ts`). On a
native-Linux Incus install this resolves via Incus's own DNS. On colima
(macOS) it does **not** resolve — you need `host.lima.internal` instead.
Integration tests override via `process.env.ZUND_HOST_API_URL`. A
production-quality fix would detect the Incus deployment at launch and
inject the right hostname.

**Path isolation for tests.** Every `~/.zund/...` path in the daemon
routes through `paths.ts::zundHome()`, which honors `ZUND_HOME` env var.
Integration tests set `ZUND_HOME=<tmp>` so they write to a throwaway
directory instead of the developer's real `~/.zund`. On macOS, setting
`$HOME` is not enough because `os.homedir()` reads `getpwuid_r()` rather
than `$HOME` — `ZUND_HOME` is the only reliable knob.

## Container layout

Every `zund-<agent>` container has this filesystem shape after launch:

```
/root/.pi/agent/
    settings.json      provider, model, skills[], enableSkillCommands
    auth.json          { <provider>: { type: "api_key", key: "..." } }
    models.json        only for custom providers (ollama, etc.)
    sessions/          ← bind-mount to ~/.zund/data/sessions/<agent>
    extensions/
        zund-fleet.ts  ← written by pi/extension-writer.ts at launch time;
                         exposes fleet identity to Pi (agentName, fleetName,
                         teamName, otherAgents) and registers tools:
                         memory_save, memory_search, working_memory_update,
                         working_memory_patch, zund_fleet_status, load_skill

/workspace             ← bind-mount to ~/.zund/data/workspace/<agent> (RW)
/skills/<skill-name>   ← bind-mount to ~/.zund/data/skills/<name> (RO)
```

Environment variables set via `setContainerConfig environment.*`:

- `ZUND_API_URL=http://host.incus:4000` (always)
- anything from `agent.secrets` in the fleet YAML

## Host data layout

```
~/.zund/
├── fleet-state.yaml            persisted desired state (one YAML doc per resource)
├── logs/zundd.log              (TODO — not yet written; daemon logs to stdout)
└── data/
    ├── memory.db               SQLite facts + working memory
    ├── sessions.db             SQLite session index
    ├── sessions/<agent>/       Pi session .jsonl files
    ├── workspace/<agent>/      agent scratch dir, mounted RW
    └── skills/<skill-name>/    copied skill directories, mounted RO
```

## API surface (non-exhaustive; see `api/server.ts` for full list)

- `GET  /health` — daemon liveness
- `GET  /v1/fleet/status` — list running agents + health summary
- `GET  /v1/fleet/export` — dump current fleetState as YAML
- `DELETE /v1/fleet` — destroy all agents + clear state
- `POST /v1/apply` — the main entrypoint. `{ path | content | resources[], preview?, force? }`
- `GET  /v1/events` — SSE stream of fleet events
- `POST /v1/agents/:name/message` — SSE streaming chat
- `POST /v1/agents/:name/stop` / `/restart`
- `GET  /v1/agents/:name/state` / `/sessions` / `/skills`
- `GET  /v1/agents/:name/memory?q=&scope=` — memory facts
- `POST /v1/agents/:name/memory { content, scope }` — saveFact (dedup + 500/scope cap)
- `DELETE /v1/agents/:name/memory?scope=&olderThanMs=` — bulk prune
- `DELETE /v1/agents/:name/memory/:id` — delete one fact
- `GET  /v1/agents/:name/working-memory?scope=`
- `PUT  /v1/agents/:name/working-memory { content, scope }` — full-doc replace
- `PATCH /v1/agents/:name/working-memory { slot, operation, content?, scope }` — H2 slot update
- `GET  /v1/agents/:name/working-memory/scopes` — enumerate scopes
- `DELETE /v1/agents/:name/working-memory/gc { scopePrefix, exceptScope }` — GC stale thread WM
- `GET  /v1/fleet/:name/working-memory` — fleet shared context (read-only)
- `GET  /v1/debug/agent/:name` — full snapshot for debugging (handle + fleet
  state + Incus devices + env + live `/root/.pi/agent/` listing + parsed
  settings.json + `/skills/` listing). Use this first when diagnosing agent
  problems — it replaces a half-dozen `incus exec` commands.

## Known quirks worth remembering

1. **`differ` doesn't cascade dependencies** — skill edits don't rebuild consumer agents. Workaround: delete + reapply.
2. **Daemon has no file log** — `~/.zund/logs/zundd.log` does not exist yet; logs go to stdout only.
3. **`GET /v1/agents/:name/skills` is a stub** — always returns `mounted: true` with `description: undefined`. Use `/v1/debug/agent/:name` instead for real data.
4. **`resolveLocalSkill` re-copies on every call** — cpSync overwrites, so re-applies pick up new skill content IF the agent is in the plan (which requires the cascade fix above).
5. **Restart: skills persist, fleetState rehydrates, handles reconnect** — but if you changed skill YAML while the daemon was down, the containers still have the old mounts. A delete+reapply is the safe reset.
6. **Pre-existing type errors** — `apps/daemon` currently has ~42 `tsc --noEmit` errors in `pi/rpc.ts`, `differ.ts`, `incus/client.ts`, and memory endpoint body types. These are unrelated to any recent feature work and need a dedicated cleanup pass.
