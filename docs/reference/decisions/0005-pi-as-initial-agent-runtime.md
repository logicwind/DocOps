---
id: "0005"
title: Pi as initial agent runtime
date: 2026-04-16
status: accepted
implementation: done
supersedes: []
superseded_by: null
related: ["0003", "0009", "0013"]
tags: [runtime, pi]
---

# 0005 · Pi as initial agent runtime

Date: 2026-04-16
Status: accepted
Evidence: POC 03 (`03-pi-in-incus/`), POC 04 (`04-rpc-proxy/`), POC 08 (`08-pi-extension-dev/`), POC 11 (`11-pi-session-lifecycle/`)
Related: ADR 0003 (runtime interface), ADR 0013 (Pi tools field)

## Context

An agent runtime is the code that executes *inside* a container — the thing
that speaks to LLMs, holds conversation state, runs tools, and streams
output to the daemon. The daemon itself is runtime-agnostic; the runtime
is what gives a container the ability to "be an agent."

Zund needed a runtime that:

- Speaks OpenRouter / OpenAI / Anthropic / Ollama out of the box
- Supports tools with a pluggable registration mechanism
- Persists conversation state to disk (not memory)
- Can be controlled from outside the container over a long-lived channel
- Works in a resource-constrained environment (Pi 4 class hardware)

## Decision

Use Pi (a proprietary LLM agent runtime) as the first agent runtime
implementation. One Pi process runs per agent, inside its container, talking
to the daemon over `incus exec`-piped stdin/stdout.

**Why Pi:**

- POC 03: runs cleanly inside an Incus container with minimal setup
- POC 04: `incus exec` long-lived process gives <1ms proxy overhead for
  stdin/stdout RPC
- POC 08: extensions can be loaded as `.ts` files via jiti; tools and
  context injection work
- POC 11: Pi's JSONL session format (v3) is parseable without the Pi SDK,
  which keeps the daemon decoupled from Pi internals

**Mechanics:**

- Daemon starts a long-lived `incus exec` process per agent.
- Extension file (`zund-fleet.ts`) is injected into the container at
  `/root/.pi/agent/extensions/` at launch time, providing Zund tools
  (`memory_save`, `memory_search`, `zund_emit_artifact`, etc.) and
  context hooks.
- Messages flow over stdin; events flow back over stdout as JSONL.
- Sessions are written to a host-mounted directory per-agent (ADR 0009).

## Consequences

**Makes easier:**

- No agent-runtime work required — Pi does the LLM harness, tool registry,
  conversation loop, session persistence for free.
- Model flexibility — Pi already supports the major providers.
- Extension model fits Zund's fleet concept cleanly.

**Makes harder:**

- Pi is proprietary. Long-term Zund open-source story requires an
  alternative runtime (or a runtime-neutral "which runtime do you want?"
  experience).
- Wire format is Pi's JSONL vocabulary (addressed by ADR 0002 stream protocol
  and ADR 0003 runtime interface).
- Container naming is Pi-aware (`zund-${agent.name}`) because of how Pi
  accesses host mounts — addressed by making container naming a runtime
  detail in ADR 0003.

## Implementation notes

- Lives at `packages/daemon/src/pi/` today; moves to
  `packages/daemon/src/agents/runtimes/pi/` under ADR 0003.
- Pi is invoked inside the pre-built `zund/base` image (ADR 0007) which
  ships Pi pre-installed for fast cold start.
- Event vocabulary (`message_update`, `tool_execution_start`, etc.) is
  Pi-native; canonical vocabulary per ADR 0002 translates it at L3.
