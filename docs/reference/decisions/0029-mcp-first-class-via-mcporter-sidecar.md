---
id: "0029"
title: "MCP as first-class fleet capability via mcporter sidecar"
date: 2026-04-18
status: draft
implementation: not-started
supersedes: []
superseded_by: null
related: ["0004", "0012", "0019", "0020", "0022", "0027", "0028", "0030"]
tags: [mcp, tools, plugins, substrate, l1, l3]
---

# 0029 · MCP as first-class fleet capability via mcporter sidecar

Date: 2026-04-18
Status: draft
Related: ADR 0004 (Incus ephemeral clone — defines the fleet network
and container shape), ADR 0012 (secrets), ADR 0019 (MCP support —
establishes direction; this ADR specifies the deployment
architecture), ADR 0020 (plugin architecture — `mcp-host` plugin
kind), ADR 0022 (stream protocol — `data-z:fleet:*` events for
sidecar lifecycle), ADR 0027 (Pi baseline extensions — MCP is
parallel to, not a replacement for, Pi's fleet-tier extensions),
ADR 0028 (fleet_capabilities — `mcp` is one of the entries),
ADR 0030 (packs — packs may contribute `mcp_servers:` entries).

## Context

ADR 0019 established that Zund supports MCP and laid out four
candidate architectures (A through D), ultimately recommending Option
B (daemon-spawned sidecars + per-runtime thin bridge) for Phase 1. It
deliberately left the exact deployment shape open.

Since 0019, two things changed:

1. **mcporter is a credible off-the-shelf implementation.**
   [steipete/mcporter](https://github.com/steipete/mcporter)
   auto-discovers and proxies MCP servers (stdio via `npx -y <pkg>`
   or HTTP), caches OAuth tokens, pools connections, and exposes a
   single HTTP endpoint. It removes 80% of the lifecycle work the
   daemon would otherwise have to own.
2. **The three-tier mental model locked in** (ADR 0028). MCP is the
   third tier — runtime-agnostic, shared across the fleet. A shared
   sidecar matches this structure exactly.

Additionally, the per-agent MCP config shape from ADR 0019 has a
practical problem: **OAuth tokens, package downloads, and connection
state duplicate per agent** if each agent container runs its own
MCP client. One shared sidecar per fleet fixes this without changing
the user-facing YAML shape much.

This ADR **extends** ADR 0019 rather than superseding it. ADR 0019's
decision ("yes to MCP, architecture TBD") stands. This ADR specifies
*how*.

## Decision

**One mcporter sidecar container per fleet**, on the fleet's Incus
network, with a shared host-wide package cache volume. Fleet YAML
gains an `mcp_servers:` block; `zund apply` reconciles the sidecar
config and restarts it. Agents bridge to the sidecar via their
runtime plugin's `bridgeFor("mcp")` (ADR 0028).

### 1. One sidecar per fleet

Named `<fleet>-mcp` (e.g., `work-mcp`, `research-mcp`). Lives on the
fleet's Incus network. Agents in the fleet resolve the sidecar via
fleet-internal DNS as `zund-mcp` (or equivalent short name).

```
┌─────────────────────── Fleet network ───────────────────────┐
│                                                              │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐                  │
│  │ agent-1  │   │ agent-2  │   │ agent-3  │                  │
│  └────┬─────┘   └────┬─────┘   └────┬─────┘                  │
│       │ HTTP         │ HTTP         │ HTTP                   │
│       └──────────────┼──────────────┘                        │
│                      ▼                                       │
│             ┌─────────────────┐                              │
│             │  zund-mcp       │  ← mcporter sidecar          │
│             │  (mcporter)     │                              │
│             └────┬─────┬──────┘                              │
│                  │     │                                     │
└──────────────────┼─────┼─────────────────────────────────────┘
                   │     │
            stdio: npx   HTTP: remote MCP
        @modelcontextprotocol/server-github     mcp.linear.app/mcp
```

### 2. Fleet YAML — `mcp_servers:` block

Fleet-level (not per-agent) declaration:

```yaml
# fleet/mcp.yaml (or inline in fleet.yaml)
mcp_servers:
  github:
    transport: stdio
    command: npx -y @modelcontextprotocol/server-github
    env:
      GITHUB_TOKEN: ref://secrets.GITHUB_TOKEN

  linear:
    transport: http
    url: https://mcp.linear.app/mcp
    auth:
      bearer: ref://secrets.LINEAR_TOKEN

  playwright:
    transport: stdio
    command: npx -y @modelcontextprotocol/server-playwright
    # no env — no secrets

  gws:
    transport: stdio
    command: uvx run google-workspace-mcp
    env:
      GOOGLE_OAUTH_CACHE: /root/.mcporter/gws
```

Secret refs resolve at apply time via the secrets plugin (ADR 0012).
The sidecar never sees `ref://` syntax.

### 3. Lazy-launch + warm-keep

mcporter spawns stdio servers on **first tool call**, not at sidecar
boot. It keeps daemons warm after. This is documented behavior; the
implication is that **first-use latency is real** (npx download on
first call of a package the host cache hasn't seen). Cold-start UX
matters — mitigations:

- Shared package cache volume (next section) reduces cold starts to
  the first fleet on the host, not every fleet.
- Operators can pre-warm packages via `zund mcp warm <server>` at
  apply time (future CLI verb; not in v1).
- The `data-z:fleet:mcp-warming` event (new per this ADR) surfaces
  cold-start latency to the console so users aren't confused by the
  first tool call taking seconds.

### 4. Shared package cache volume

A persistent Incus volume bind-mounted into every fleet's sidecar at
`/root/.npm` and `/root/.cache/uv`. The cache is **host-wide** (one
per zundd install), not per-fleet.

```
host:/var/lib/zund/mcp-cache/
  ├── npm/
  │   └── _cacache/    ← shared npm package cache
  └── uv/
      └── ...          ← shared uv package cache
```

Every fleet's sidecar mounts this volume read-write at the same
path. First fleet to use `@modelcontextprotocol/server-github` pays
the download; every other fleet on the host reuses it. Cache
invalidation is the default npm/uv behavior (semver + lockfiles);
Zund does not second-guess it.

### 5. Credential flow

**At apply time:**

- Secret refs in `mcp_servers:` resolve via the secrets plugin.
- Resolved values written into the sidecar config file (mcporter's
  native YAML, per its schema).

**At runtime, for OAuth-capable servers:**

- OAuth tokens land in sidecar `~/.mcporter/<server>/` inside the
  sidecar's **per-fleet config volume** (separate from the shared
  package cache — OAuth tokens are fleet-scoped, not host-wide).
- Tokens survive sidecar restarts because the config volume is
  persistent.
- First-time OAuth flows use the `zund auth <pack>` wizard (ADR 0030;
  forward-looking reference).

**At runtime, for key-based servers:**

- Env vars passed to the stdio process at spawn. No file write.

### 6. Agent-side integration (via runtime bridge)

Per ADR 0028, `fleet_capabilities: [mcp]` triggers the runtime
plugin's `bridgeFor("mcp")`. The bridge configures the runtime to
point at `http://zund-mcp:<port>/` for tool discovery and call
routing.

**Pi:** the bridge generates an extension at
`~/.pi/extensions/zund-fleet/mcp-bridge.ts` that uses mcporter's HTTP
API for listing tools and invoking them. The extension registers one
Pi tool per discovered MCP tool, prefixed by server name
(`github.create_issue`, `linear.list_issues`).

**Hermes:** the bridge writes an entry into Hermes's native
`~/.hermes/config.yaml` pointing at `http://zund-mcp:<port>/`.
Hermes's built-in MCP client handles the rest.

**OpenClaw:** same pattern — its native MCP client gets pointed at
the sidecar.

In every case, the agent sees MCP tools as native tools. The
sidecar is invisible at the tool-call layer.

### 7. Lifecycle

**`zund apply` sidecar reconciliation:**

1. Compute the union of `mcp_servers:` from fleet YAML **plus** any
   `mcp_servers:` contributed by enabled packs (ADR 0030).
2. Render mcporter's native config file.
3. If config changed, restart the `<fleet>-mcp` sidecar.
4. Validate: attempt a `listTools` call against each declared
   server; surface failures on the apply report.

**No image rebuild per config change** — configs are rendered and
injected at apply time; MCP servers fetched on-demand at first use.
This is the key property over a "bake MCP servers into the agent
image" alternative.

**Sidecar health** surfaces on `/v1/fleet/:name/status` as a
first-class resource:

```json
{
  "fleet": "work",
  "mcp_sidecar": {
    "status": "healthy",
    "servers": [
      { "name": "github", "status": "warm" },
      { "name": "linear", "status": "cold" },
      { "name": "playwright", "status": "warming" }
    ]
  }
}
```

New wire events (extends ADR 0022's `data-z:fleet:*` catalog):

```
data-z:fleet:mcp-sidecar-starting   { fleet }
data-z:fleet:mcp-sidecar-ready      { fleet, servers }
data-z:fleet:mcp-sidecar-failed     { fleet, error }
data-z:fleet:mcp-warming            { fleet, server }
data-z:fleet:mcp-unavailable        { fleet, server, reason }
```

**Sidecar crash does not take down agents.** Agents degrade to "no
MCP tools available" and emit `data-z:fleet:mcp-unavailable` on
their next tool-call attempt. Failed tool calls return a clean error
to the LLM (not a hang), so the agent can route around or surface
the problem.

### 8. Relationship to ADR 0019's per-agent MCP config

ADR 0019 proposed a per-agent `mcp: [names]` field on role YAML.
**That shape is deprecated by this ADR.** Per-agent MCP config does
not work well with a shared sidecar:

- OAuth tokens would duplicate across agents of the same fleet that
  use the same server.
- Shared package cache can't dedupe across agents with different
  version pins.
- Concurrent access to the same stdio process needs coordination
  that's more naturally done at the sidecar layer.

The new model: `mcp_servers:` is fleet-level. Every agent with
`fleet_capabilities: [mcp]` sees every server in the fleet's sidecar.
If an operator wants per-agent scoping, they run separate fleets
(which already have separate sidecars). Per-agent allow-lists of
specific tools may return as a v2 concern if real multi-tenant
requirements appear — call that out in open questions.

ADR 0019's four-option analysis stands; this ADR picks a concrete
shape within Option B using mcporter as the implementation.

## Challenges and open questions

### Cold-start latency

npx-fetched packages take seconds on first use per host. The
shared cache fixes this after the first fleet; it does not fix the
very first tool call after a fresh install. Operators of tight
demos should pre-warm. Not a v1 blocker.

### Per-tool rate limits across the shared sidecar

If agent-1 and agent-2 both call `github.create_issue` concurrently,
they share the GitHub token's rate budget. This is **correct** (the
token is fleet-scoped) but can be surprising. Document it.

### Isolation between agents

A malicious MCP server could, in principle, return prompt-injection
content consumed by multiple agents in the same fleet. The sidecar
does not sanitize; that's the runtime's job (agents trust tool
results). This is not a regression — it's the same threat model as
ADR 0019.

### Sidecar resource limits

The sidecar is a single container with many spawned subprocesses.
Memory budget per server and overall sidecar cgroup limits need
documentation; reasonable defaults in v1 (e.g., 1GB RSS for the
sidecar, 256MB per spawned stdio server) with override via
`mcp_servers: { <name>: { resources: { mem: "512Mi" } } }`.

### mcporter updates

mcporter is a third-party dependency. Pin the version via the
sidecar image (produced by Zund, versioned alongside the daemon).
Breaking changes in mcporter trigger a sidecar image bump; existing
fleets keep running the pinned version until operator upgrade.

### Remote MCP servers and the sidecar

For `transport: http` servers (remote MCP endpoints), the sidecar
doesn't run a subprocess — it proxies. The shared cache, lazy-launch,
and warm-keep mechanics don't apply. The sidecar still centralizes
auth and audit for these, which is the value-add.

### Tool name collisions

Two MCP servers exposing `create_issue` collide. mcporter handles
this via server-prefixed names (`github.create_issue`,
`linear.create_issue`). Pi bridge preserves the prefix in the Pi
tool name. Document.

## Consequences

**Makes easier:**

- **Zero image rebuild per config change.** Adding a new MCP server
  is an `apply`, not an image rebuild + push + roll.
- **Shared package cache amortizes cost.** First fleet to pull
  playwright-mcp pays; every other fleet on the host reuses.
- **OAuth tokens persist across sidecar restarts.** One auth flow
  per server, not per agent.
- **Clean fit with ADR 0028.** `fleet_capabilities: [mcp]` is one
  entry; the bridge wires the rest.
- **Packs contribute MCP cleanly.** ADR 0030 packs union into
  `mcp_servers:` with no per-pack lifecycle concerns.
- **Sidecar crash is isolated.** Agents keep running; MCP tools
  degrade; state clears on restart.

**Makes harder:**

- **New container type to manage.** `<fleet>-mcp` adds a sidecar
  per fleet. Daemon orchestration grows by this type.
- **Cold-start latency is user-visible.** First tool call after a
  fresh install can take seconds. Documented, mitigable, but real.
- **Multi-fleet environments share the npm/uv cache.** Any risk
  from a malicious package propagates host-wide, not just
  fleet-wide. Mitigated by pinning and lockfiles; worth a note in
  the threat model.
- **mcporter is a supply-chain dependency.** Pinned, but a trust
  decision.

## Relationship to existing ADRs

| ADR | Relationship |
|-----|-------------|
| 0004 | The sidecar runs on the fleet's Incus network; ephemeral clone mechanics apply. |
| 0012 | `mcp_servers:` uses `ref://secrets.X` syntax, resolved by the secrets plugin at apply. |
| 0019 | This ADR extends 0019 by specifying the deployment architecture (Option B with mcporter). Per-agent `mcp: [names]` shape from 0019 is deprecated in favor of fleet-level `mcp_servers:`. |
| 0020 | mcporter-sidecar is a concrete impl of the `mcp-host` plugin kind. Alternative impls (e.g., a custom host) satisfy the same contract. |
| 0022 | New `data-z:fleet:mcp-*` events extend the catalog. Sidecar state is first-class on the wire. |
| 0027 | MCP is parallel to Pi's fleet-tier extensions, not a replacement. Pi extensions are latency-critical + fleet-stateful; MCP is commodity + third-party. |
| 0028 | `fleet_capabilities: [mcp]` triggers the runtime's `bridgeFor("mcp")`, which points the runtime at the sidecar. |
| 0030 | Packs contribute `mcp_servers:` entries; the apply step unions pack contributions into fleet YAML contributions. |

## Implementation notes

**New daemon module:**

```
packages/daemon/src/mcp/
  sidecar-manager.ts    ← spawn, restart, health for <fleet>-mcp
  config-render.ts      ← fleet YAML + packs → mcporter config
  sidecar-image.ts      ← pinned mcporter image spec
```

**New plugin package:**

```
packages/plugins/mcp-host-mcporter/   ← default mcp-host impl
  # thin wrapper; real work is in the sidecar image
```

**Sidecar image:**

- Base: minimal Node/Bun + Python (for uvx) image.
- Pinned mcporter version.
- Bind mounts: host cache volume (`/root/.npm`, `/root/.cache/uv`)
  and per-fleet config volume (`/root/.mcporter/`).
- Runs as a non-root user inside the container where feasible.

**Fleet parser changes:**

```
packages/daemon/src/fleet/parser.ts
  - accept top-level mcp_servers: block
packages/daemon/src/fleet/types.ts
  - FleetResource gains mcp_servers?: Record<string, McpServerSpec>
```

**Apply path addition:**

After existing apply steps, before container start:

1. Union fleet YAML `mcp_servers:` + pack contributions.
2. Resolve `ref://secrets.*`.
3. Render mcporter config.
4. Reconcile sidecar container (create / restart if config changed).
5. Validate `listTools` against each server; report failures.

**Runtime plugin changes:**

- `runtime-pi` (ADR 0027): `bridgeFor("mcp")` generates the mcp-bridge
  extension pointing at `http://zund-mcp:<port>/`.
- Future runtimes (Hermes, OpenClaw): their `bridgeFor("mcp")`
  writes the runtime's native MCP config pointing at the same URL.

**No changes to:** service-tier plugins, the wire protocol core
(ADR 0022 base), the task queue (ADR 0023), the docs store
(ADR 0025). The `data-z:fleet:*` catalog grows; that is a
v1-additive extension per ADR 0022's compatibility policy.

## Next steps

- Spike the sidecar image with one real MCP server
  (`@modelcontextprotocol/server-github`) end-to-end from fleet YAML
  → sidecar config → Pi agent tool call.
- Measure cold-start latency on a fresh host and document.
- Define the `zund mcp warm` CLI verb (deferred to post-v1 but
  scope the design).
- Align pack authoring guide (ADR 0030) with the `mcp_servers:`
  merge semantics.
