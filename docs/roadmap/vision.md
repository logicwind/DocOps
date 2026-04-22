# Zund — Product Vision

> Orchestration infrastructure for fleets of specialized AI agents.

## One-liner

Define your agent fleet in YAML. `zund apply` launches a coordinated
team — containerized, observable, with curated capability packs.

## Core thesis

**A fleet of specialized agents beats a monolithic generalist.**

Hermes and OpenClaw ship as heavyweight single-agent experiences with
60–120 bundled skills each — the iPhone of agents. Zund's wedge is the
inverse: lightweight Pi agents, specialized per role, coordinated
through fleet primitives and furnished with curated capability packs.

This is not a better agent. It is a **network** of agents. For users
who want the monolith for a specific role, they load Hermes or
OpenClaw as a fleet member. Zund subsumes them without competing — it
is the layer above.

## Who this is for

Self-hosted teams and companies. Install with a command, point at a
fleet folder, apply. The on-ramp is a curated pack set — a new user's
first successful fleet looks like:

```yaml
# fleet.yaml
runtime: pi
packs:
  - research-primitives
  - github-workflow
  - team-ops
```

Three lines, real working fleet. Expanding from there is adding more
agents, more packs, more fleets — never more infrastructure.

**Goal:** 100 teams using it.

## What Zund is NOT

- **Not a better agent.** Zund does not replace Hermes, OpenClaw, or
  Claude Cowork. It runs them as fleet members.
- **Not a coding assistant.** Pi is general-purpose; coding is one
  workload among many. No opinion about IDE integration.
- **Not a multi-node cluster scheduler.** Single-host is the design
  target; federation is commercial.
- **Not an MCP server directory.** MCP discovery and curation belong
  elsewhere; Zund only runs the servers you configure.
- **Not a replacement for Kubernetes.** Incus is the substrate.

## Product decisions (not technical decisions)

Technical decisions live in ADRs; this table is strategic only.

| Decision | Why |
|---|---|
| **YAML is desired state; `zund apply` reconciles** | Git-backable fleets. No surprises, no auto-watch. |
| **Multi-runtime from day one** | Pi default; Hermes/OpenClaw as alternates. Don't bet on any single runtime. |
| **Fleet capabilities are Zund-owned; commodity capabilities are runtime-internal** | Zund owns only what must be cross-agent (queue, delegation, shared stores). Everything runtime-native stays runtime-native. |
| **Packs are the distribution unit** | Curated skill + MCP + secrets bundles. "Batteries included" without us writing 100 skills. |
| **MCP is first-class** | One mcporter sidecar per fleet. Any third-party integration with a credible MCP server is reachable by every agent. |
| **Everything goes through the daemon** | CLI, console, scripts, channels — all thin clients of one API. |

## Business model (open core)

| Tier | What | License |
|---|---|---|
| **Open Source** | zundd, CLI, SDK — single fleet, teams, packs, MCP sidecar | MIT / Apache 2.0 |
| **Zund Pro** | Multi-fleet, RBAC, audit, cost tracking, clustering | Source-available |
| **Zund Console** | Hosted web dashboard, workflow builder, pack marketplace | SaaS |
| **Zund Cloud** | Managed zundd + Incus hosting | SaaS |

## How this vision is realized

This file is the *why* and the *what*. The *how* lives elsewhere:

- **Architecture** — layer map, component responsibilities, flows:
  see `reference/architecture.md`.
- **Mental model** — vocabulary, concept relationships, decision
  trees: see `reference/mental-model.md`.
- **Diagrams** — system architecture + fleet anatomy:
  see `reference/diagrams.md`.
- **Decisions** — every material technical choice recorded as an ADR:
  `reference/decisions/`.
- **Active work** — the current slice and the near-term parking lot:
  `roadmap/current.md` and `roadmap/next.md`.

If this vision doc contradicts those: the code is the spec, ADRs are
the record, this is the compass. Compasses should be updated when
strategy shifts — not when the implementation does.
