# Zund Mental Model

The concept relationship map. **Read this if you want to understand how
Zund works, not just where the code lives.**

- For the picture (system architecture + fleet anatomy mermaid diagrams), see `diagrams.md`.
- For the layer view (L1–L4, rate of change), see `architecture.md`.
- For per-decision rationale, see `decisions/NNNN-*.md` ADRs.
- For wire format, see `runtime-protocol.md`.

This doc is the vocabulary. Every term Zund uses appears here with one
clear definition, its relationships to other terms, and the invariants
that keep the model coherent. If you find yourself confused about
"skill vs tool vs extension vs MCP vs pack," start here.

---

## Part 1 — The product thesis in one paragraph

A **fleet of specialized agents beats a monolithic generalist.**
Hermes and OpenClaw ship as heavyweight single-agent experiences with
60–120 bundled skills each. Zund's wedge is the inverse: lightweight
Pi agents, specialized per role, coordinated through **fleet primitives**
(task queue, dispatcher, delegation, shared memory, shared docs, shared
MCP sidecar) and furnished with **curated capability packs** (skill +
MCP + secrets bundles). Parallelism + specialization + curation > any
single monolith. For users who want the monolith, Zund runs Hermes or
OpenClaw as a fleet member — it does not compete with them, it subsumes
them.

Everything in this document exists to make that thesis real.

---

## Part 2 — The 4-layer architecture (brief recap)

Authoritative in `architecture.md` and ADR 0001. Summary:

```
L4  ACCESS         CLI · Console · REST · SSE · Channels     changes with UX
L3  ORCHESTRATION  Dispatcher · Queue · Triggers · MCP       changes with product
L2  STATE          Secrets · Memory · Artifacts · Sessions   changes with schemas
L1  SUBSTRATE      Incus · Containers · Fleet reconciler     stable
```

An **Agent Runtime Interface** sits between L1 and L3. Runtimes (Pi,
Hermes, OpenClaw) are swappable implementations of that interface.

This document is about what happens **across** these layers from an
agent's point of view. The 4-layer model is "how Zund is built." The
capability model that follows is "what Zund exposes."

---

## Part 3 — The three capability tiers (orthogonal to L1–L4)

When an agent does work, the tools it calls come from one of three
places:

```
┌────────────────────────────────────────────────────────────┐
│ FLEET TIER  (Zund-owned, cross-agent state)                │
│                                                            │
│   memory · artifacts · docs · fleet-status                 │
│   task-delegate · MCP-via-sidecar                          │
│                                                            │
│   → every runtime MUST bridge to these to be a             │
│     first-class fleet citizen                              │
└────────────────────────────────────────────────────────────┘
                    ↕ bridge (runtime-specific)
┌────────────────────────────────────────────────────────────┐
│ RUNTIME TIER  (runtime-internal)                           │
│                                                            │
│   Pi:     extensions + plugin kinds + builtin tools        │
│   Hermes: toolsets + plugins + native skills               │
│   OpenClaw: skills + MCP + terminal backends               │
│                                                            │
│   → commodity capabilities (web-search, browser, terminal, │
│     email, etc.) live here, configured via                 │
│     `runtime_config:` pass-through (ADR 0028)              │
└────────────────────────────────────────────────────────────┘
                    ↕ mcporter sidecar (ADR 0029)
┌────────────────────────────────────────────────────────────┐
│ MCP TIER  (runtime-agnostic, shared across fleet)          │
│                                                            │
│   GitHub · Linear · Notion · Slack · Google Workspace      │
│   Playwright · Stripe · Arxiv · custom MCPs                │
│                                                            │
│   → external processes exposing tools via MCP protocol     │
│   → one mcporter sidecar per fleet, lazy-launched, cached  │
└────────────────────────────────────────────────────────────┘
```

### Why three tiers

**Fleet tier** is what makes Zund "a fleet" instead of "some agents."
These capabilities depend on state no single agent can own: the shared
memory store, the fleet-wide artifact registry, the list of sibling
agents, the task queue. Every runtime plugin implements a bridge — the
mechanism differs (Pi generates extension files; Hermes writes plugin
entries; OpenClaw drops skill configs), but the interface the agent
sees is uniform.

**Runtime tier** is what the runtime already ships. Hermes's
`web_tools.py` is 2,100 lines with 4 search backends, LLM compression,
URL safety, managed tool-gateway — trying to abstract that into Zund
contracts would either lose fidelity or reimplement everything. Same
for terminal backends (Hermes has 6: local/docker/ssh/singularity/
modal/daytona), browser integration, email clients. Zund stays out.
Fleet YAML's `runtime_config:` block writes into the runtime's native
config file verbatim at container boot (with secret refs resolved).

**MCP tier** is the rest of the world. The industry is converging on
MCP for third-party integrations; rather than building native wrappers
for GitHub/Linear/Notion/Slack/GWS/Playwright, Zund runs an mcporter
sidecar per fleet that hosts any MCP servers the fleet needs. Agents
discover these tools through the same mechanism they discover any
other tool. No rebuild per config change — configs are injected at
apply time and MCP servers are fetched on-demand via `npx -y <pkg>`.

### The decision: where does capability X belong?

```
Is X a fleet primitive                    ──YES──▶  FLEET TIER
(cross-agent state, Zund-unique)?                   (bridge in runtime plugin)
  │
  NO
  ▼
Is X latency-critical AND small?          ──YES──▶  RUNTIME TIER
(e.g. web-search: called every step,               (Pi extension, runtime builtin)
tiny HTTP wrapper, fits in-process)
  │
  NO
  ▼
Does a credible MCP server exist?         ──YES──▶  MCP TIER
(GitHub, Linear, Notion, GWS, Playwright,          (mcporter sidecar)
official or community, maintained)
  │
  NO
  ▼
Is X a process or workflow pattern,       ──YES──▶  SKILL
(how to do a thing, not code)?                     (pack: SKILL.md)
  │
  NO
  ▼
Skip or defer. Need more evidence that X is worth the surface area.
```

---

## Part 4 — The four extensibility primitives

Skills, tools, extensions, MCP servers. These are **not competing** —
they are complementary mechanisms that compose into the capability
tiers above. Get the relationships right and the rest falls out.

### Tool — the atomic unit

A **tool** is a callable function the LLM can invoke. It has:

- a name (e.g. `web_search`, `github_create_pr`, `delegate_task`)
- a JSON schema for arguments
- an implementation (code that runs when the LLM calls it)

The LLM only ever sees tools. It does not know or care where a tool
comes from. To the LLM, `web_search` is `web_search` whether it is
provided by a Pi extension, an MCP server, or a runtime builtin.

### Extension — code that registers tools in the runtime

An **extension** (Pi's term; other runtimes use other names) is a
TypeScript file loaded by the runtime at startup. It exports a set of
tool handlers that the runtime registers with the LLM.

- **In-process.** Runs inside the agent's container, in the same
  process as the LLM loop. Low latency.
- **Generated, not hand-written.** For Zund-controlled bridges, the
  runtime plugin's boot code (`packages/plugins/runtime-pi/src/
  extension.ts`) emits the extension files pointing at the active
  plugins in the registry. For user-authored extensions, the runtime's
  own extension loader handles them.
- **Best for:** fleet primitives (`memory-bridge`, `task-delegate`),
  latency-critical commodities (`web-search`).

Pi's seven extensions are the canonical set — see ADR 0027. Hermes has
its own extension mechanism (TS modules for sub-agents, plan modes,
etc.). OpenClaw has skills-as-extensions via SKILL.md with code blocks.

### MCP server — external process exposing tools via the MCP protocol

An **MCP server** is a separate process (often started via
`npx -y <package>` or an HTTP endpoint) that exposes a set of tools
over the Model Context Protocol. An **MCP client** in the agent
(or in Zund's mcporter sidecar) connects to the server and surfaces
its tools to the LLM.

- **Out-of-process.** Runs in a sidecar container or remote endpoint.
- **Language-agnostic.** MCP servers are written in any language. Most
  are Node or Python.
- **Lazy.** mcporter doesn't require pre-installed servers; they are
  fetched on-demand and cached.
- **Stateful-capable.** mcporter keeps warm daemons for stateful
  servers (chrome-devtools-mcp, playwright-mcp).
- **Best for:** third-party integrations, anything with an existing
  credible MCP server (GitHub, Linear, Notion, Slack, GWS,
  Playwright), untrusted code (isolated process).

### Skill — markdown document describing a workflow

A **skill** is a markdown file (by convention `SKILL.md`) that
describes a workflow or pattern. It is **pure prose.** No code.

- Loaded on-demand via the runtime's `load_skill` tool. The LLM asks
  for a skill by name when it needs that workflow.
- References tools by name: *"To open a PR, use `github_create_pr` with
  the reviewer list from `linear_get_issue`."*
- Does not provide tools. A skill whose referenced tools are not
  registered is broken.
- Can declare dependencies in frontmatter (required tools, required
  packs) so loading fails loudly.

Pi and Hermes both have `load_skill` mechanics. OpenClaw's skills are
directories with SKILL.md plus tool handlers; same idea with a
different loader.

**Why skills exist:** encoding process and style knowledge that does
not fit cleanly into tool schemas. "How we write PR descriptions."
"The firm's meeting-note template." "The 5-step incident response
runbook." These are prompts, not APIs.

### The invariants — what keeps this coherent

1. **The LLM only sees tools.** Skills are prose loaded into context;
   MCP servers and extensions both register tools with identical
   shapes. The LLM's mental model is "I have these tools; this skill
   tells me how to use them for task X."

2. **Skills are prose, not code.** A skill can *reference* tools it
   depends on but never *provides* them. Skills + tools are two
   layers, not two flavors of the same thing.

3. **Extension and MCP server are alternative sources of tools.**
   Both end up as "a tool the LLM can call." Pick by latency,
   isolation, language, and whether a third party already wrote one.

4. **A toolset is not a skill.** A toolset is a **named bundle of
   tools to enable/disable** (manifest-level selector). A skill is
   text describing a workflow. Orthogonal — Hermes happens to group
   tools into toolsets AND ship skills; don't collapse the concepts.

5. **Runtime tools are a black box to Zund.** Pi's `load_skill`,
   Hermes's `web_search`, OpenClaw's shell — Zund does not interpret,
   only passes runtime config through.

---

## Part 5 — Packs: the distribution unit

A **pack** is an opt-in bundle combining:

- one or more **skills** (SKILL.md files)
- zero or more **MCP server** configurations
- the **secret keys** those MCP servers require

Packs answer the question: *how does a user get "batteries included"
without us writing 100 skills?* Each pack is a curated, quality-gated
capability area. The v1 set:

| Pack | Skills | MCP servers | Why |
|---|---|---|---|
| `research-primitives` | web-search patterns, arxiv flow, summarization | Tavily, arxiv-mcp | The research agent on day one |
| `github-workflow` | PR workflow, code review, issue triage | `@modelcontextprotocol/server-github` | Dev-team magnet |
| `productivity-gws` | Gmail, Calendar, Drive patterns | community GWS MCP | Consumer/ops magnet |
| `team-ops` | Linear, Notion, Slack patterns | linear-mcp, notion-mcp, slack-mcp | Where teams live |
| `docs-io` | PDF extract, OCR, long-doc summarization | unstructured-mcp or similar | Document-heavy workflows |
| `browser-automation` | Navigate, login, extract patterns | playwright-mcp | Agents that actually use the web |

### Pack lifecycle

On `zund apply`:

1. Daemon reads each listed pack manifest (`pack.yaml`).
2. **Skills:** skill `.md` files copy into the agent's `skills/`
   workspace where the runtime's `load_skill` tool finds them.
3. **MCP servers:** pack MCP configs union into the fleet's mcporter
   sidecar config; sidecar restarts.
4. **Secrets:** required secrets validated via the active secrets
   plugin. Missing secret → apply fails with clear "pending" state
   (per ADR 0023).
5. Emit `data-z:fleet:pack-loaded` event.

### Pack distribution

- **Bundled** — `packages/packs/` in the zund monorepo.
- **Contrib** — npm packages `@zund/pack-*`, installed via
  `zund pack install <name>`.
- **User-local** — `packs/` in the fleet folder (git-tracked for
  private team workflows).

See ADR 0030 for full manifest shape, resolution semantics, and the
promotion path from contrib to bundled.

---

## Part 6 — Where configuration lives

Zund's config surface has four files/blocks, each with one purpose.
Knowing which is which removes most confusion.

```
~/.zund/plugins.yaml
  ───────────────────
  Global plugin selection.
  Which memory impl, which secrets impl, which web-search backend.
  One choice per plugin kind. Persists across fleets.

  plugins:
    memory: sqlite
    secrets: age-sops
    web-search: tavily
    docs: local


fleet/<agent-name>.yaml
  ─────────────────────
  Per-agent declarative config. Zund interprets this.

  name: researcher
  runtime: pi
  fleet_capabilities:               # Zund-owned. Every one bridged.
    memory: enabled
    artifacts: enabled
    docs: enabled
    task-delegate: enabled
    fleet-status: enabled
  packs:                            # Opt-in capability bundles.
    - research-primitives
    - github-workflow
  runtime_config: { ... }           # See below. Opaque to Zund.
  mcp_servers: { ... }              # Extra MCP servers beyond packs.


fleet/<agent-name>.yaml — runtime_config block
  ───────────────────────────────────────────
  Runtime-native configuration. Opaque pass-through.
  Zund writes this verbatim into the runtime's native config file
  at container boot, with secret refs resolved.

  runtime_config:
    # For Pi: written to ~/.pi/config.toml
    extensions:
      web-search: { backend: tavily }
    # For Hermes: written to ~/.hermes/config.yaml
    toolsets: [web, terminal, file]
    web:
      backend: exa
      exa_api_key: ref://secrets.EXA_API_KEY
    terminal:
      backend: docker
      docker_image: python:3.11-slim


packs/<pack-name>/pack.yaml
  ─────────────────────────
  Pack manifest — skill + MCP + secrets bundle.

  name: github-workflow
  skills:
    - path: skills/github-pr-workflow/SKILL.md
    - path: skills/github-code-review/SKILL.md
  mcp_servers:
    - name: github
      transport: stdio
      command: npx -y @modelcontextprotocol/server-github
      env: { GITHUB_TOKEN: ref://secrets.GITHUB_TOKEN }
  secrets_required: [GITHUB_TOKEN]
```

**Key property:** `runtime_config` is opaque to Zund. Zund never
interprets Hermes's `terminal.backend`, Pi's extensions list, or
OpenClaw's skill registry config. Each runtime plugin knows its
runtime's native config shape and passes it through.

**Secret refs** (`ref://secrets.X`) resolve via the active secrets
plugin at apply time. The runtime never sees the ref syntax.

---

## Part 7 — End-to-end: how a tool call reaches the LLM and back

Concrete trace for `web_search("latest Claude API changes")`.

### Setup (once, at `zund apply`)

1. Parse `fleet/researcher.yaml`. See `runtime: pi`,
   `fleet_capabilities: [memory, artifacts, docs, task-delegate,
   fleet-status]`, `packs: [research-primitives]`.
2. Resolve `research-primitives` pack: copy skill files, union MCP
   configs, validate secrets.
3. Resolve `runtime_config`. Tavily backend selected; `TAVILY_API_KEY`
   resolved from secrets.
4. **Pi runtime plugin's bridge step:** for each `fleet_capability`,
   generate an extension file in the container's
   `~/.pi/extensions/zund-core/`:
   - `memory-bridge.ts` → calls daemon HTTP for the active memory
     plugin
   - `task-delegate.ts` → calls `POST /v1/tasks`
   - `web-search.ts` → calls daemon HTTP for the active web-search
     plugin (Tavily) with API key from env
5. Container boots. Pi loads extensions. Pi connects to the mcporter
   sidecar at `http://zund-mcp:<port>/`. Tools from pack MCP servers
   (Tavily-as-MCP? no, Tavily is a Pi extension; the sidecar hosts the
   research pack's arxiv-mcp and whatever else).
6. Pi advertises tool list to the LLM. `web_search` is among them.

### Call flow (every user message)

```
User: "Look up latest Claude API changes."
  │
  ▼
LLM decides to call: web_search(query="latest Claude API changes")
  │
  ▼
Pi's tool dispatcher → generated web-search.ts extension
  │
  ▼
HTTP POST to daemon: /v1/capabilities/web-search/search
  │
  ▼
Daemon resolves active `web-search` plugin (Tavily).
Tavily plugin hits https://api.tavily.com/search with the query
and the resolved API key.
  │
  ▼
Tavily returns pre-summarized snippets + URLs.
  │
  ▼
Daemon emits `tool-call-start` → `tool-call-delta` → `tool-result`
UIMessage parts on the fleet stream (ADR 0022).
  │
  ▼
Pi receives result, surfaces to LLM as tool return.
  │
  ▼
LLM composes response using snippets. Streams `text-delta` back.
  │
  ▼
Console (via AI Elements) + channel adapters render the stream.
```

For a different capability like `github_list_issues`:

```
LLM: github_list_issues(repo="zund/zund", state="open")
  │
  ▼
Pi's tool dispatcher → NOT a Zund extension.
  (github is registered via the mcporter sidecar, not as a Pi extension.)
  │
  ▼
Pi's MCP client forwards the call to zund-mcp sidecar on fleet network.
  │
  ▼
mcporter routes to the github MCP server process
(spawned on first use via `npx -y @modelcontextprotocol/server-github`
with GITHUB_TOKEN env var).
  │
  ▼
GitHub MCP calls api.github.com with the token.
Results flow back through mcporter to Pi.
  │
  ▼
Same UIMessage stream, same console rendering.
```

Same observable behavior from the LLM's perspective. Different plumbing
underneath.

---

## Part 8 — Where things live in code (conceptual map)

```
packages/core/src/contracts/
  memory.ts · artifacts.ts · secrets.ts · sessions.ts       (L2 contracts)
  docs.ts                                                    (ADR 0025)
  web-search.ts                                              (ADR 0027, new kind)
  media-stt.ts · media-tts.ts · media-vision.ts              (ADR 0031)
  queue.ts · dispatcher.ts                                   (ADR 0023)
  runtime.ts                                                 (ADR 0003 — adds
                                                             configPath,
                                                             configFormat,
                                                             bridgeFor — ADR 0028)
  events.ts                                                  (ADR 0022 — UIMessagePart
                                                             + data-z:* catalog)

packages/plugins/
  runtime-pi/                  ← primary Pi runtime plugin
    src/extensions/            ← the seven extension generators (ADR 0027)
      web-search.ts
      web-fetch.ts
      memory-bridge.ts
      artifacts-bridge.ts
      docs-bridge.ts
      fleet-status.ts
      task-delegate.ts
  runtime-hermes/              ← future (ADR 0018 Phase 2)
  runtime-openclaw/            ← future
  memory-sqlite/               ← default memory impl
  artifacts-local/             ← default artifacts impl
  secrets-age-sops/            ← default secrets impl
  docs-local/                  ← default docs impl (ADR 0025)
  web-search-tavily/           ← default web-search impl (ADR 0027)
  web-search-brave/            ← alternative web-search impl
  media-stt-whisper/           ← future (ADR 0031)
  media-tts-elevenlabs/        ← future

packages/packs/
  research-primitives/         ← v1 pack (ADR 0030)
  github-workflow/
  productivity-gws/
  team-ops/
  docs-io/
  browser-automation/

apps/daemon/src/
  incus/ · fleet/              ← L1 substrate (ADR 0001)
  secrets/ · memory/ · artifacts/ · sessions/     ← L2 state
  api/                         ← L4 access
  queue/ · dispatcher/ · triggers/                 ← L3 orchestration (ADR 0023)
  mcp/                         ← mcporter sidecar lifecycle (ADR 0029)
  packs/                       ← pack resolver (ADR 0030)
  stream/                      ← UIMessage + data-z:* translator (ADR 0022)
  capability/                  ← fleet_capabilities registry (ADR 0028)
```

---

## Part 9 — Decision trees

### "I want to add a new capability."

```
1. Is it a Zund-unique fleet primitive (cross-agent state, queue,
   delegation, shared stores)?
   ── YES → Pi extension under runtime-pi/src/extensions/.
            Add to fleet_capabilities vocabulary.
            Other runtimes must add their own bridge.
   ── NO  → Go to 2.

2. Is it a widely-used commodity with a credible MCP server?
   (GitHub, Linear, Notion, Slack, Google Workspace, Playwright, ...)
   ── YES → Add to an appropriate pack (or a new pack).
            Never build native.
   ── NO  → Go to 3.

3. Is it latency-critical and small (called every step, trivial impl)?
   (web-search, web-fetch)
   ── YES → Pi extension. New plugin kind in core.
            Ship at least one default backend.
   ── NO  → Go to 4.

4. Is it a process/pattern/workflow rather than code?
   ── YES → Skill (SKILL.md). Ship as part of a pack if it needs
            tools that aren't already available.
   ── NO  → Go to 5.

5. Is it already native to a runtime (Hermes's terminal backends,
   Pi's load_skill)?
   ── YES → Nothing to do. Users configure it via runtime_config:
            pass-through in fleet YAML.
   ── NO  → Defer. Not enough evidence to justify surface area.
```

### "What's the difference between X and Y?"

- **Skill vs tool.** Skill is prose describing a pattern. Tool is a
  callable function with a schema. Skills reference tools by name;
  skills do not provide tools.

- **Tool vs extension.** Tool is the thing the LLM calls. Extension
  is the code mechanism that registers a tool with the runtime. One
  extension can register many tools.

- **Extension vs MCP server.** Both register tools with the LLM, but
  extensions run in-process (low latency, language = runtime's
  language) while MCP servers run out-of-process (isolation, any
  language, sidecar-hosted). Pick by latency, isolation, and whether a
  third party already built one.

- **Toolset vs skill.** Toolset is a named bundle of tools to enable.
  Skill is a markdown workflow description. Orthogonal.

- **Pack vs skill.** Pack is a distribution unit bundling skills + MCP
  servers + required secrets. A pack contains skills; it is not a
  skill.

- **Fleet capability vs runtime capability.** Fleet capability is
  Zund-owned and every runtime must bridge to it. Runtime capability
  is runtime-internal and configured via `runtime_config:`
  pass-through.

- **`plugins.yaml` vs `runtime_config:`.** `plugins.yaml` selects one
  impl per Zund plugin kind (memory, secrets, web-search, etc.).
  `runtime_config:` is opaque pass-through into the runtime's native
  config. Zund interprets the first; it does not interpret the second.

- **`fleet_capabilities:` vs `packs:` vs `mcp_servers:` in fleet YAML.**
  `fleet_capabilities` enumerates which fleet primitives this agent
  bridges to (Zund-owned list). `packs` opts into curated bundles.
  `mcp_servers` declares individual MCP servers outside any pack.

---

## Part 10 — Glossary

One-line definitions. Skim to refresh; deep-dive above if unclear.

- **ADR** — Architecture Decision Record. `docs/reference/decisions/NNNN-*.md`.
- **Agent** — An LLM loop + tools, running inside a container, governed by a runtime.
- **Artifact** — A content-addressed blob emitted by an agent (output files, images, etc.). See ADR 0011 / 0025.
- **Bridge** — Code that makes a fleet capability accessible to a runtime's LLM. Runtime-specific mechanism (Pi: extension; Hermes: plugin entry; OpenClaw: skill).
- **Capability tier** — One of {fleet, runtime, MCP}. The three places an agent's tools come from.
- **Channel adapter** — L4 component that translates the `zund://stream/v1` wire into a messaging platform's native messages (Slack, Telegram, WhatsApp, etc.). See ADR 0022 §10.
- **Contract** — A TypeScript interface in `@zund/core/contracts/` defining a plugin kind's shape (MemoryStore, WebSearchProvider, etc.).
- **data-z:** — Wire-event namespace for Zund-native concepts on top of AI SDK UIMessage (ADR 0022). e.g. `data-z:task:completed`, `data-z:fleet:apply-started`.
- **Dispatcher** — L3 component that routes tasks to agents via a small LLM call (ADR 0023).
- **Docs store** — Unified artifact + knowledge store per ADR 0025. Replaces the old "artifacts vs knowledge" split.
- **Extension** — TS code the runtime loads at startup that registers tools in-process. Pi's extension system is the reference.
- **Fleet** — A set of agents defined by YAML files in a fleet folder. `zund apply` reconciles.
- **Fleet capability** — A Zund-owned primitive every runtime must bridge: `memory`, `artifacts`, `docs`, `fleet-status`, `task-delegate`. See ADR 0028.
- **Fleet YAML** — A YAML file per agent under the fleet folder. Zund-interpreted config.
- **LLM** — Large language model inside the agent loop. The LLM is what calls tools.
- **MCP** — Model Context Protocol. Standard for external tool servers. See ADR 0019, 0029.
- **mcporter** — Node-based MCP client/runner that hosts multiple MCP servers via `npx -y <pkg>` or HTTP. Zund runs one per fleet in a sidecar. See ADR 0029.
- **Pack** — Distribution unit bundling skill(s) + MCP server(s) + required secrets. Opt-in per agent. See ADR 0030.
- **Pi** — The default agent runtime. Lightweight, TS-based, container-native. See ADR 0005. Pi-specific `/tools` and `/extensions` mechanisms are runtime-tier concerns.
- **Plugin** — An implementation of a contract. Named `<kind>-<backend>` (`memory-sqlite`, `web-search-tavily`). Swappable via `~/.zund/plugins.yaml`.
- **Plugin kind** — A contract category. `memory`, `artifacts`, `secrets`, `sessions`, `runtime`, `docs`, `web-search`, `media-stt`, etc.
- **`plugins.yaml`** — `~/.zund/plugins.yaml`. Global plugin selection — one impl per kind.
- **`runtime_config:`** — Opaque pass-through block in fleet YAML. Written verbatim to the runtime's native config file at container boot, with secret refs resolved. See ADR 0028.
- **Runtime** — An implementation of the Runtime interface (ADR 0003): Pi, Hermes, OpenClaw. Each has its own native config format and extensibility story.
- **Runtime plugin** — A `@zund/plugin-runtime-<name>` package implementing the Runtime contract. Only the daemon imports these.
- **Sidecar** — An auxiliary container attached to a fleet. Zund ships one per fleet for MCP (mcporter); future sidecars may host browser (Camofox) or media providers.
- **Skill** — Markdown document (SKILL.md) describing a workflow or pattern. Prose, not code. Loaded on-demand via `load_skill`.
- **Stream protocol** — `zund://stream/v1` = AI SDK UIMessage base + `data-z:*` Zund extensions. See ADR 0022.
- **Task** — A work item with free-text prompt, optional hints, traveling through the task queue. See ADR 0023.
- **Tool** — A callable function the LLM can invoke. Name + schema + handler. The atomic unit of agent action.
- **Toolset** — A named bundle of tools to enable/disable. Runtime-level concept (Hermes has toolsets; Pi's equivalent is per-agent tool lists).
- **zundd** — The daemon. Runs continuously; manages fleet state, API, Incus, sidecars, sessions. All business logic lives here.

---

## Part 11 — How to extend this doc

- **This doc is the vocabulary, not the decisions.** If you find
  yourself writing "we chose X because Y," that belongs in an ADR.
- **Keep part 4 (the four primitives) stable.** The invariants at the
  end of part 4 are the load-bearing beams. Don't soften them.
- **Update the component map (part 8) when packages move.** Drift here
  misleads every reader.
- **Add to the glossary (part 10) before introducing a new term in
  an ADR.** If a concept does not have a one-line definition, it is
  not yet clear enough to decide with.
- **The product thesis (part 1) changes only with a top-level strategy
  decision.** Reflect vision-doc changes here; do not invent positioning.
