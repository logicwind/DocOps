# The magic path — foundation + capability pivot

Status: **active**
Goal: get to the magic — a working multi-agent fleet demonstrably more
capable than any single monolithic runtime, with curated packs that
make lightweight Pi feel batteries-included.

Related ADRs: 0022 (stream protocol), 0027 (Pi baseline extensions),
0028 (runtime config + fleet_capabilities), 0029 (MCP sidecar),
0030 (packs), 0023 (task queue — Phase 1 within this slice).

---

## Why this is the slice

Earlier `current.md` was ADR 0020 Phase 3 (memory-postgres contrib
plugin). That work is plumbing with no user-visible magic — it lives
in the parallel lane (see `next.md`) and is not blocking.

The three-tier capability model (fleet / runtime / MCP) landed as
ADRs 0027–0031 on 2026-04-18. Implementing them, plus the messaging
contract that renders their output, is what turns the architectural
pivot into a visible product.

---

## Sequenced deliverables

Each section has a concrete **ship criterion** — the thing that must be
true for the step to be considered done. Skipping ahead is allowed
(steps can overlap), but no step is "done" until its criterion holds.

### 1. Messaging contract (ADR 0022 implementation)

Without this, everything downstream emits events into a void. This is
the foundation that lets tools, packs, and tasks render.

- `@zund/core/src/contracts/events.ts` — re-export AI SDK
  `UIMessagePart` + `ZundDataPart` discriminated union for every
  `data-z:*` variant in ADR 0022 §3 (agent, artifact, memory, task,
  fleet).
- `packages/plugins/runtime-pi/src/stream/translator.ts` — stop being
  identity. Emit real `UIMessagePart` + `data-z:agent:*` /
  `data-z:artifact:*` / `data-z:memory:updated`.
- New endpoint: `GET /v1/tasks/:id/stream` (ADR 0022 §6).
- `apps/console` — install `ai` + `@ai-sdk/react` + AI Elements
  registry. Replace custom chat render with `useChat`. Four
  `data-z:*` renderers: ArtifactCreatedCard, MemoryUpdatedBadge,
  TaskStatePill (placeholder), AgentLifecycleToast.

**Ship criterion:** console renders a Pi agent's tool call with a
proper Artifact card, Chain-of-Thought collapse, and Memory badge. No
custom streaming code left in the console.

### 2. Runtime config + fleet_capabilities (ADR 0028 implementation)

The architectural seam that makes commodity capabilities runtime-internal
and Zund-owned capabilities fleet-level.

- Fleet YAML parser: `runtime_config:` (opaque pass-through) +
  `fleet_capabilities:` (Zund-owned list).
- Runtime plugin contract additions: `configPath`, `configFormat`,
  `bridgeFor(capability)`.
- Pi runtime's `bridgeFor()` wires the 7 fleet capabilities from
  ADR 0027.
- Secret-ref resolution (`ref://secrets.X`) at apply time; never
  written to disk in plaintext.
- Deprecate ADR 0013's top-level `tools:` field in favor of
  `runtime_config: { extensions: ... }`.

**Ship criterion:** a single fleet YAML with both `runtime: pi` and
`runtime: hermes` agents applies cleanly, each getting its own native
config format, sharing the same `fleet_capabilities` contract. (Hermes
plugin itself is not required yet — a stub runtime that reads the
config works.)

### 3. Pi baseline extensions (ADR 0027 implementation)

The seven extensions that make a Pi agent useful out of the box.

- `@zund/core/src/contracts/web-search.ts` — `WebSearchProvider`.
- `packages/plugins/web-search-tavily/` — default backend.
- `packages/plugins/web-search-brave/` — alternative.
- `packages/plugins/runtime-pi/src/extensions/` — generators for
  `web-search`, `web-fetch`, `task-delegate`. `memory-bridge`,
  `artifacts-bridge`, `fleet-status` stay (or relocate into the same
  folder if they aren't there yet).
- `docs-bridge` generator — per ADR 0025 `docs` plugin kind.

**Ship criterion:** a fresh Pi agent with no pack declared can answer
"what's new with Claude APIs?" via `web_search` + `web_fetch`, save
results via `memory_save`, and the console shows the whole flow as
proper UIMessage parts.

### 4. MCP sidecar (ADR 0029 implementation)

One mcporter sidecar per fleet. Lazy-launched MCP servers. No image
rebuilds per config change.

- `apps/daemon/src/mcp/` — sidecar lifecycle (start/config/restart).
- Shared host-wide npm + uv cache volume.
- Fleet YAML `mcp_servers:` block → sidecar config on apply.
- Per-fleet OAuth config volume at `~/.mcporter/` inside the sidecar.
- Sidecar health on `/v1/fleet/status`; crash → degrade to "no MCP
  tools" with `data-z:fleet:mcp-unavailable`.
- Deprecate ADR 0019's per-agent `runtime.mcp` config.

**Ship criterion:** a fleet YAML declaring `mcp_servers: [github]`
starts a sidecar, agents discover `github_*` tools, and an agent can
successfully call `github_list_issues` in a demo repo. The sidecar
survives agent restarts.

### 5. Packs: first two bundled (ADR 0030 implementation)

The distribution unit that makes lightweight Pi batteries-included.
Ship two to prove the mechanism; add the other four in `next.md`.

- `packages/packs/` scaffold.
- Pack manifest parser (`pack.yaml`).
- Pack resolver: skill files → agent workspace, MCP configs → sidecar
  union, secrets → validate.
- Fleet YAML `packs:` block.
- First two bundled packs:
  - **`research-primitives`** — web-search patterns, arxiv-mcp,
    summarization skill.
  - **`github-workflow`** — PR workflow, code review, issue triage
    skills + `@modelcontextprotocol/server-github`.

**Ship criterion:** `zund apply` on a fleet with
`packs: [research-primitives, github-workflow]` produces a working
researcher + coder pair. The researcher can find and summarize a
paper; the coder can triage a real GitHub issue.

### 6. Task queue — Phase 1 (ADR 0023 implementation)

The primitive that makes the `task-delegate` extension from step 3
actually do something.

- `@zund/core/src/contracts/queue.ts` — `TaskQueue` contract.
- `packages/daemon/src/queue/sqlite-queue.ts` — `SqliteTaskQueue`.
- Routes: `POST /v1/tasks` (with `hints.agent` direct assignment only
  in Phase 1), `GET /v1/tasks`, `GET /v1/tasks/:id`,
  `DELETE /v1/tasks/:id`, `POST /v1/tasks/:id/result`.
- `data-z:task:*` wire events on every state transition (subset in
  Phase 1: `queued` → `dispatched` → `running` → terminal).
- Console `TaskStatePill` renderer (stub from step 1) goes live.

**Ship criterion:** agent A calls `delegate_task(agent="coder",
prompt="...")`, the task appears in the console queue, agent B picks
it up, completes it, result flows back to A via artifact store + SSE.
End-to-end multi-agent demo works.

---

## Ship criterion for the whole slice

**A three-agent fleet demo** runs end-to-end:

```yaml
# fleet.yaml
packs:
  - research-primitives
  - github-workflow

agents:
  - name: researcher
    runtime: pi
    fleet_capabilities: [memory, artifacts, docs, fleet-status, task-delegate]
  - name: planner
    runtime: pi
    fleet_capabilities: [memory, artifacts, docs, fleet-status, task-delegate]
  - name: coder
    runtime: pi
    fleet_capabilities: [memory, artifacts, docs, fleet-status, task-delegate]
```

User prompts `planner`: "*Find the three most-commented open issues in
repo X, summarize the top one, draft a PR description, and assign
research to the researcher.*"

planner uses `delegate_task` to send research to researcher, calls
GitHub MCP tools directly for issue listing, delegates PR drafting
to coder. Console shows the task graph, artifacts as cards,
tool calls as expandable Chain of Thought.

This is the magic demo. When it works, the slice ships.

---

## Not in this slice (→ `next.md`)

- ADR 0023 Phase 2 (LLM dispatcher for free-text task routing)
- ADR 0023 Phase 3 (cron/webhook/agent-chain triggers)
- Remaining four v1 packs (productivity-gws, team-ops, docs-io,
  browser-automation)
- Channel adapters (Slack/Telegram/WhatsApp — ADR 0022 §10)
- ADR 0031 media capabilities
- Hermes runtime plugin (ADR 0018 Phase 2)
- ADR 0020 Phase 3 (memory-postgres contrib plugin — parallel lane)
