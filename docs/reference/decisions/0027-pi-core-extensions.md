---
id: "0027"
title: "Pi runtime baseline extensions — the seven fleet-bridged capabilities"
date: 2026-04-18
status: accepted
implementation: partial
supersedes: []
superseded_by: null
related: ["0005", "0012", "0013", "0019", "0020", "0022", "0023", "0025", "0028", "0029", "0030"]
tags: [pi, runtime, extensions, tools, capabilities, general-purpose-agent]
---

# 0027 · Pi runtime baseline extensions — the seven fleet-bridged capabilities

Date: 2026-04-18
Status: draft
Related: ADR 0005 (Pi as initial runtime), ADR 0012 (secrets), ADR 0013
(Pi tools field), ADR 0019 (MCP support), ADR 0020 (plugin architecture),
ADR 0022 (stream protocol), ADR 0023 (task queue + dispatcher),
ADR 0025 (docs / unified artifact + knowledge store),
ADR 0028 (runtime config pass-through + fleet_capabilities contract),
ADR 0029 (MCP first-class via mcporter sidecar),
ADR 0030 (packs as capability bundle distribution unit)

## Context

Earlier drafts of this ADR tried to answer a broader question: "what
commodity capabilities does every Pi agent need out of the box?" and
proposed a confirmed-plus-evaluate split that included browser, email,
calendar, messaging, and notes. That framing collapsed once the
three-tier capability model crystallized (ADR 0028):

1. **Fleet tier** — Zund-owned primitives every runtime must bridge.
   Agents reach these by name; the runtime plugin wires the bridge.
2. **Runtime tier** — the runtime's own configuration surface (Pi's
   extensions, Hermes's toolsets + web backends, OpenClaw's skill
   registry). Opaque to Zund, passed through verbatim (ADR 0028).
3. **MCP tier** — runtime-agnostic external tools delivered via a
   shared mcporter sidecar per fleet (ADR 0029).

Most of the "evaluate" items from the prior draft belong in tiers 2 or
3, not tier 1:

- `browser` → MCP via `playwright-mcp` in the sidecar (ADR 0029). Not
  a Pi-native extension in v1. Promote to a Camofox sidecar later if
  anti-bot stealth matters.
- `email`, `calendar`, `messaging`, `notes` → runtime-native or MCP
  territory. Each has at least one credible MCP server (Gmail via GWS
  MCP, Slack MCP, etc.); the few without (WhatsApp voice) belong to
  the channel-adapter layer in ADR 0022 §10, not the agent tool
  surface. Where skill-level workflows are wanted, they ship as packs
  (ADR 0030).

What's left is the tight set of **fleet-tier** capabilities: the tools
a general-purpose Pi agent needs that (a) depend on fleet state the
runtime alone cannot see, or (b) are too latency-sensitive to route
through the MCP sidecar. Those stay as in-process Pi extensions.

Core tension remains: each extension is attack surface and cognitive
load on the prompt. The goal is **minimal** (fewest extensions that
unlock real work) and **powerful** (each extension earns its place).
Seven extensions, no more.

## Decision

Ship a fixed, opinionated set of **seven core extensions** with every
Pi runtime plugin install. Extensions are code-generated at container
boot alongside the existing fleet bridges
(`packages/plugins/runtime-pi/src/extension.ts`). Each either wraps an
existing fleet primitive (ADR 0028 `fleet_capabilities`) or resolves
to a bound service plugin (ADR 0020).

Core extensions are **enabled by default** and **disable-able per-agent**
in fleet YAML. They are not magic — they use the secrets plugin
(ADR 0012) for credentials, the stream protocol (ADR 0022) for
rendering, and (for `task-delegate`) the daemon's HTTP surface.

### The seven — confirmed v1 set

Collapsing the prior "3 confirmed + 5 evaluate" framing, this single
table is the confirmed set:

| # | Extension | Purpose | Default backend | Swap path | Status |
|---|---|---|---|---|---|
| 1 | `web-search` | Search the web, return ranked snippets + URLs | Tavily API | plugin kind `web-search` (tavily, brave) | NEW |
| 2 | `web-fetch` | Fetch URL → clean markdown | local (jsdom + turndown) | — (in-process, no plugin) | NEW |
| 3 | `memory-bridge` | Read/write fleet memory | ADR 0020 `memory` plugin | via `plugins.yaml` | EXISTS |
| 4 | `artifacts-bridge` | Emit/read artifacts | ADR 0020 `artifacts` plugin (→ `docs` per 0025) | via `plugins.yaml` | EXISTS |
| 5 | `docs-bridge` | Read/write the unified docs store | ADR 0025 `docs` plugin kind | via `plugins.yaml` | NEW (per 0025) |
| 6 | `fleet-status` | Discover sibling agents + capabilities | daemon HTTP (`/v1/fleet/*`) | — | EXISTS |
| 7 | `task-delegate` | Queue a task for another fleet agent | `POST /v1/tasks` (ADR 0023) | — (queue is a plugin kind) | NEW |

Everything outside this list is out of scope for the Pi extension
layer. See "Explicitly moved out" below for pointers.

### Why Pi-native, not MCP, for these seven

The mcporter sidecar (ADR 0029) is the right home for commodity
third-party tools. The seven above are not commodity:

- **Latency-critical.** `web-search` and `web-fetch` are called on
  almost every agentic step. Routing them through a sidecar (spawn on
  first use, cross-network JSON-RPC, shared resource contention)
  measurably degrades interactive UX for a capability that is trivial
  to implement locally.
- **Deep fleet integration.** `memory-bridge`, `artifacts-bridge`,
  `docs-bridge`, `fleet-status`, `task-delegate` all depend on
  fleet-level state (the active `memory` plugin, the agent registry,
  the task queue). No single-agent MCP server can know these. Per ADR
  0028 they are `fleet_capabilities`, bridged per runtime.
- **Agent identity.** `artifacts-bridge`, `docs-bridge`, and
  `task-delegate` write with the agent's identity. Routing through a
  shared sidecar complicates auth; the in-process bridge carries the
  identity naturally.

### Why Tavily is the default web-search backend

Prior draft: Brave as default, Tavily as alternative. Revised: Tavily
as default, Brave as the cheap alternative.

- **Tavily ships agent-ready snippets.** Its API returns
  pre-summarized, de-duplicated text blocks intended for LLM
  consumption. No LLM compression layer needed — drop the results
  into the agent's tool result and move on.
- **Brave returns SERP-shaped results.** Good quality, but each hit is
  a title + snippet + URL; consuming the full snippet set without an
  intermediate compression step burns tokens and hurts retrieval. To
  use Brave well, Zund would have to build a Gemini/OpenRouter-style
  compression layer — exactly the 2,100-line web_tools.py path Hermes
  ended up on. Out of scope for a default.
- Brave remains the first-class alternative (`plugin-web-search-brave`)
  for operators who already have a key or need the ~2,000 free queries
  per month. Runtime configuration selects one; both satisfy the same
  `WebSearchProvider` contract.

### Explicitly moved out

These were on the prior draft's "evaluate" list. They move to other
ADRs:

- **`browser`** → ADR 0029. Routed via `playwright-mcp` in the mcporter
  sidecar. Not a Pi extension in v1. Promote to a dedicated Camofox
  sidecar later if anti-bot requirements escalate.
- **`email`**, **`calendar`** → ADR 0030 packs. Ship as `productivity-gws`
  (Gmail + Calendar + Drive) using the community Google Workspace MCP
  server. OAuth flow handled by the `zund auth` wizard (forward-looking,
  per ADR 0029).
- **`messaging`** (Slack, Telegram, WhatsApp for agent-side sending) →
  two paths. For outbound agent tool use, ADR 0030 `team-ops` pack
  (Slack MCP). For platform-level channel adapters (WhatsApp voice
  note egress, etc.), ADR 0022 §10.
- **`notes`** → redundant. `docs-bridge` (extension 5) is the notes
  tool. A separate `notes` kind would duplicate ADR 0025's store.

### Also not core (unchanged from prior draft)

- Filesystem write/edit, shell execution — already Pi built-ins.
- Code execution sandbox (Python/Node eval) — coding-agent territory.
- GitHub / GitLab / Linear / Jira — ADR 0030 packs.
- DevOps tools (kubectl, docker, ssh, terraform) — coding-agent
  territory or runtime-native via ADR 0028 `runtime_config:`.
- oh-pi-style DX extensions (footer, diagnostics, watchdog, scheduler,
  worktree) — laptop-interactive ergonomics, not fleet-runtime needs.
- Apple suite, blockchain, gaming, red-team skills — Zund is web-first
  and container-first. Not in scope.

### Extension architecture

Core extensions are generated at container boot by
`runtime-pi/src/extension.ts` into `~/.pi/extensions/zund-core/`,
alongside the existing `zund-fleet/` bridges. Each extension module
exports:

```typescript
// Generated file shape
export const tools: Record<string, PiTool> = {
  web_search: { /* ... */ },
  web_fetch: { /* ... */ },
  // ...
};

export const events: PiEventHandler[] = [
  /* session_start, before_agent_start hooks */
];
```

New contract file introduced by this ADR:

```
packages/core/src/contracts/web-search.ts    ← WebSearchProvider
```

The other six extensions reuse existing contracts:

- `web-fetch` — no contract; in-process implementation.
- `memory-bridge` — `MemoryStore` (ADR 0015/0016).
- `artifacts-bridge` — `ArtifactStore` / `DocsStore` (ADR 0011/0025).
- `docs-bridge` — `DocsStore` + `DocsSearch` (ADR 0025).
- `fleet-status` — internal daemon API, no plugin contract.
- `task-delegate` — `TaskQueue` via daemon HTTP (ADR 0023).

Default plugin packages introduced here:

```
packages/plugins/web-search-tavily/      ← default
packages/plugins/web-search-brave/       ← alternative
# web-fetch has no plugin — fully in-process
# task-delegate has no plugin — wraps daemon HTTP
# memory/artifacts/docs/fleet-status use existing plugins
```

The runtime-pi plugin's `init()` queries the registry for each
capability kind and emits the extension code pointing at the bound
impl (exactly like `memory-bridge.ts` does today). Zero hardcoded
backends in the generator.

### Configuration

Default: all seven core extensions enabled. Fleet YAML overrides
per-agent:

```yaml
# fleet/researcher.yaml
name: researcher
fleet_capabilities: [memory, artifacts, docs, fleet-status, task-delegate]
extensions:
  web-search: { backend: brave }     # override plugin choice
  web-fetch: enabled
```

`fleet_capabilities` is defined in ADR 0028 and drives which bridges
the runtime plugin's `bridgeFor()` mounts. The six fleet-tier
extensions (2–7) are inert when their corresponding capability is not
listed; the agent simply does not see those tools. `web-search` and
`web-fetch` are Pi-local and governed by the `extensions:` toggle map
alone.

Plugin backends come from `~/.zund/plugins.yaml`:

```yaml
plugins:
  web-search: tavily
```

Credentials resolve via the secrets plugin (ADR 0012). Tavily declares
its required secret key (`TAVILY_API_KEY`); the daemon binds it at
container launch through the pass-through mechanism in ADR 0028.

### Prompt visibility

Tools appear in the agent's system prompt via the existing
`before_agent_start` hook, grouped by category:

```
Available tools:
  [Web]      web_search, web_fetch
  [Memory]   memory_save, memory_search
  [Docs]     docs_put, docs_get, docs_search
  [Fleet]    fleet_status, delegate_task
```

Per-agent disabled extensions don't appear in the prompt — the agent
does not know it doesn't have them, which keeps prompts minimal.

## Sequencing — where this slots

This ADR sits between the messaging-contract slice (ADR 0022
implementation) and ADR 0023 Phase 1. `task-delegate` is the bridge
from Pi into the task queue; it is what makes multi-agent demos
possible.

1. **Messaging-contract slice** (ADR 0022 implementation)
   - `@zund/core` types the `ZundDataPart` catalog
   - Pi translator emits real `UIMessagePart` + `data-z:*`
   - Console migrates to AI Elements + `useChat`

2. **Pi runtime baseline extensions** (this ADR)
   - `web-search` (Tavily) + secret wiring
   - `web-fetch` (jsdom + turndown, in-process)
   - `memory-bridge`, `artifacts-bridge`, `docs-bridge`,
     `fleet-status` — already exist or land with ADR 0025
   - `task-delegate` — thin wrapper over `POST /v1/tasks`
     (lights up once ADR 0023 Phase 1 lands)

3. **ADR 0023 Phase 1** — task queue with `hints.agent` direct
   dispatch. Lights up `data-z:task:*` events and activates
   `task-delegate`.

4. **ADR 0029 mcporter sidecar + ADR 0030 packs** — the commodity
   capability surface (browser, GitHub, Linear, GWS, Slack) lands as
   packs, not core extensions.

5. **ADR 0023 Phase 2** — LLM dispatcher. Free-text task → routed to
   the right agent. Multi-agent magic.

## Consequences

**Makes easier:**

- **Out-of-box magic.** `zund apply` + one agent + a prompt = real
  action. The demo story collapses from "first install these 5 skills"
  to "here, apply this fleet."
- **Tight prompt surface.** Seven tools, curated, always work. Not
  100 skills the agent might ignore.
- **Clean tier separation.** Commodity stuff is MCP (ADR 0029) or pack
  (ADR 0030); runtime-internal stuff is pass-through (ADR 0028);
  fleet-unique stuff is in this list. Each tier has one owner.
- **Provider competition.** `web-search: tavily | brave` is a
  one-liner in `plugins.yaml`. Same pattern as memory / artifacts /
  secrets.
- **`task-delegate` unlocks multi-agent.** The first demo that
  distinguishes Zund from Hermes/OpenClaw.

**Makes harder:**

- **Boot-time surface still grows.** Every extension is attack
  surface. Kept small by capping at seven. New extensions require an
  ADR amendment or a new ADR.
- **Coupling to ADR 0023.** `task-delegate` is inert until the queue
  exists. Acceptable: it emits a "queue not available" error and
  auto-disables.
- **Coupling to ADR 0025.** `docs-bridge` requires the `docs` plugin
  kind. Until 0025 ships, `artifacts-bridge` serves the write path
  and `docs_search` is unavailable.

## Relationship to existing ADRs

| ADR | Relationship |
|-----|-------------|
| 0005 | Pi is the runtime; this ADR enriches Pi's default extension set. |
| 0012 | `web-search` reads `TAVILY_API_KEY` via the secrets plugin. |
| 0013 | The `tools:` field in agent YAML selects which Pi built-ins are active; this ADR adds a parallel `extensions:` field for core extension toggles. |
| 0019 | MCP remains parallel to core extensions — user-configured vs guaranteed. See ADR 0029 for the deployment architecture. |
| 0020 | `web-search` is a new plugin kind. Default (Tavily) and alternative (Brave) both satisfy the contract. |
| 0022 | Extension tool calls emit `tool-call-start` / `tool-result` UIMessage parts. `docs-bridge` writes emit `data-z:memory:updated` or `data-z:artifact:created`. `task-delegate` emits `data-z:task:queued` via the daemon. |
| 0023 | `task-delegate` is a thin wrapper around `POST /v1/tasks` — depends on queue existing. |
| 0025 | `docs-bridge` is the agent's surface onto the unified docs store. Supersedes the prior draft's "notes" extension. |
| 0028 | The six fleet-tier extensions (2–7) are mounted via `bridgeFor(capability)` for each `fleet_capabilities:` entry. `web-search` and `web-fetch` are Pi-local and governed by `extensions:`. |
| 0029 | Commodity web capabilities (browser, GitHub, etc.) live in the mcporter sidecar, not as Pi extensions. |
| 0030 | Skill-level workflows (email, calendar, messaging, domain-specific patterns) ship as packs, not as extensions. |

## Implementation notes

**New contract files:**

```
packages/core/src/contracts/web-search.ts
```

**New default plugin packages:**

```
packages/plugins/web-search-tavily/
packages/plugins/web-search-brave/
```

**Extension generator changes:**

```
packages/plugins/runtime-pi/src/extensions/
  web-search.ts      ← generates ~/.pi/extensions/zund-core/web-search.ts
  web-fetch.ts
  docs-bridge.ts     ← new per ADR 0025
  task-delegate.ts   ← new per ADR 0023
  # memory-bridge, artifacts-bridge, fleet-status already exist
```

The existing `extension.ts` grows a loop that, for every capability
bound via `fleet_capabilities:` plus the Pi-local `extensions:`
toggles, invokes the matching generator. Pattern mirrors the existing
`bridges/{memory,artifacts,fleet}-bridge.ts` layout.

**Fleet YAML type additions:** `AgentResource.extensions` — per-agent
toggle map for Pi-local extensions (`web-search`, `web-fetch`).
`fleet_capabilities` (ADR 0028) governs the six fleet-tier bridges.

**No changes to:** Pi core, the runtime contract (ADR 0003), the
daemon HTTP API surface (extensions reach the daemon via existing
endpoints).

## Next steps

- Validate the seven-extension scope with the team.
- Spike `web-search` (Tavily) end-to-end: contract → plugin → extension
  generator → agent prompt. Fastest proof of the full path.
- Wire `docs-bridge` against ADR 0025's `DocsStore` as soon as that
  plugin ships.
- Wire `task-delegate` against ADR 0023 Phase 1 as soon as the queue
  lands.
- Slot implementation into `roadmap/current.md` after the
  messaging-contract slice.
