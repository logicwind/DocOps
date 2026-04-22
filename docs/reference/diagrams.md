# Zund Diagrams

Two mermaid diagrams that make the system visible. Render in any
mermaid-capable viewer (GitHub, Fumadocs, mermaid.live).

- **System architecture** — how the moving parts fit: daemon, runtimes,
  sidecars, state, clients. One-page read.
- **Fleet anatomy** — what a running fleet looks like after `zund apply`:
  declarative inputs, per-fleet shared services, per-agent bridges.

For the layer-by-layer walkthrough, see `architecture.md`. For the
vocabulary and concept relationships, see `mental-model.md`.

---

## System architecture

How the daemon, fleet network, state, and clients connect. Grouped by
role; arrows show control + data flow.

```mermaid
flowchart TB
    subgraph clients["L4 — Access"]
        cli[zund CLI]
        console[Web Console<br/>AI Elements + useChat]
        channels[Channel Adapters<br/>Slack · Telegram · WhatsApp]
        script[External scripts<br/>+ webhooks]
    end

    subgraph zundd["zundd — the daemon (L3 + L2)"]
        api[REST + SSE API<br/>zund://stream/v1<br/>UIMessage + data-z:*]
        reconciler[Fleet Reconciler]
        queue[(Task Queue<br/>SQLite)]
        dispatcher[LLM Dispatcher]
        triggers[Triggers<br/>cron · webhook · chain]
        registry[Runtime Registry]
        packres[Pack Resolver]
        mcpmgr[MCP Sidecar Manager]
    end

    subgraph state["L2 — State (pluggable)"]
        secrets[(Secrets<br/>age+sops)]
        memory[(Memory<br/>sqlite + FTS5)]
        artifacts[(Artifacts<br/>CAS)]
        docs[(Docs<br/>unified store)]
        sessions[(Sessions<br/>per-runtime)]
    end

    subgraph fleet["Fleet Network (Incus, per-fleet)"]
        direction LR
        agent1["Agent: researcher<br/>runtime: pi"]
        agent2["Agent: coder<br/>runtime: pi"]
        agent3["Agent: planner<br/>runtime: hermes"]
        mcp{{mcporter sidecar<br/>GitHub · Linear · Notion<br/>GWS · Playwright · …}}
    end

    substrate[(L1 Substrate<br/>Incus · containers · networks)]

    cli --> api
    console --> api
    channels --> api
    script --> api

    api --> reconciler
    api --> queue
    queue --> dispatcher
    triggers --> queue
    dispatcher --> registry
    packres --> mcpmgr
    mcpmgr --> mcp
    registry -. runtime plugin .-> agent1
    registry -. runtime plugin .-> agent2
    registry -. runtime plugin .-> agent3

    reconciler --> substrate
    substrate --> fleet

    agent1 -. fleet bridges .-> memory
    agent1 -. fleet bridges .-> artifacts
    agent1 -. fleet bridges .-> docs
    agent2 -. fleet bridges .-> memory
    agent3 -. fleet bridges .-> memory

    agent1 -- MCP tool calls --> mcp
    agent2 -- MCP tool calls --> mcp
    agent3 -- MCP tool calls --> mcp

    api --> secrets
    api --> memory
    api --> artifacts
    api --> docs
    api --> sessions
```

**How to read:**

- **Access layer** talks only to `zundd`. No client ever touches a
  container, an MCP server, or a state store directly.
- **zundd** owns the fleet: reconciling YAML to running containers,
  routing tasks, streaming events, resolving packs, managing the MCP
  sidecar lifecycle.
- **Fleet network** is a per-fleet Incus network. Every agent (of any
  runtime) plus one shared mcporter sidecar lives here.
- **State stores** are pluggable (ADR 0020). Defaults are SQLite +
  local CAS + age/sops; swap via `~/.zund/plugins.yaml`.
- **Fleet bridges** (dotted) are the capability contract from ADR 0028:
  every runtime plugin wires its agents through to memory, artifacts,
  docs, fleet-status, task-delegate.

---

## Fleet anatomy

What `zund apply` turns YAML into. Same fleet, two views: left is what
you declare; right is what the daemon materializes.

```mermaid
flowchart LR
    subgraph declared["Declarative (fleet folder)"]
        direction TB
        fleetY["fleet/*.yaml<br/>runtime · fleet_capabilities<br/>runtime_config · packs<br/>mcp_servers"]
        packs_dir["packs/<br/>(SKILL.md + pack.yaml<br/>+ MCP configs)"]
        secrets_enc["secrets/<br/>age+sops encrypted"]
        plugins_yaml["~/.zund/plugins.yaml<br/>(global kind→impl binding)"]
    end

    apply([zund apply])

    subgraph materialized["Materialized (running fleet)"]
        direction TB

        subgraph agents_box["Agents (containers)"]
            direction LR
            a1["researcher<br/>runtime-pi<br/>7 fleet bridges<br/>+ pack skills in /skills<br/>+ web-search extension"]
            a2["coder<br/>runtime-pi<br/>7 fleet bridges<br/>+ pack skills in /skills"]
            a3["planner<br/>runtime-hermes<br/>bridges via Hermes plugins<br/>+ native toolsets"]
        end

        subgraph fleet_shared["Fleet-shared services"]
            direction LR
            mcpsrv{{"zund-mcp<br/>mcporter sidecar<br/>(union of all packs'<br/>+ explicit mcp_servers)"}}
            qsrv[(task queue)]
            msrv[(memory store)]
            asrv[(artifacts / docs)]
            ssrv[(sessions)]
        end
    end

    fleetY --> apply
    packs_dir --> apply
    secrets_enc --> apply
    plugins_yaml --> apply

    apply -- starts --> a1
    apply -- starts --> a2
    apply -- starts --> a3
    apply -- configures --> mcpsrv

    a1 -. bridge: memory .-> msrv
    a1 -. bridge: artifacts/docs .-> asrv
    a1 -. bridge: task-delegate .-> qsrv
    a1 -- MCP tool calls --> mcpsrv
    a2 -. bridge: memory .-> msrv
    a2 -- MCP tool calls --> mcpsrv
    a3 -. bridge: memory .-> msrv
    a3 -- MCP tool calls --> mcpsrv

    a1 --- ssrv
    a2 --- ssrv
    a3 --- ssrv
```

**How to read:**

- **Left side** is everything a team checks into git: fleet YAML, pack
  manifests (skill + MCP + secrets), encrypted secret files, and the
  global plugin selection. Human-editable, reviewable.
- **`zund apply`** is the boundary between declared and materialized.
  It reads the left side, resolves secret refs, unions pack MCP configs
  into sidecar config, copies skill files into each agent's workspace,
  and starts/updates containers.
- **Right side** is what runs: one container per agent plus one
  sidecar per fleet for MCP. All agents share the fleet's state stores
  and the mcporter sidecar, via runtime-specific bridges.
- **Bridges vs MCP calls.** Fleet capabilities (memory, artifacts,
  docs, task-delegate) are bridged in-process by the runtime plugin
  (low latency, deep integration). MCP-tier capabilities (GitHub,
  Linear, Playwright, …) are called through the shared sidecar.
- **Runtimes are interchangeable.** The `planner` agent runs Hermes
  instead of Pi; the fleet contract is identical. Its `runtime_config:`
  pass-through configures Hermes's toolsets and backends natively; its
  fleet bridges are implemented by the Hermes runtime plugin rather
  than Pi's extension generators.

---

## When the diagrams drift

Update this file the same PR that changes the wire, adds a plugin
kind, or renames a fleet primitive. The mental model doc's glossary
should agree with every node label in these diagrams — if it doesn't,
one of them is wrong.
