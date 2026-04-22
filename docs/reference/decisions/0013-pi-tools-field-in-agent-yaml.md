---
id: "0013"
title: "Pi built-in tool selection via `tools:` field in agent YAML"
date: 2026-04-16
status: accepted
implementation: not-started
supersedes: []
superseded_by: null
related: ["0003", "0005"]
tags: [runtime, pi, yaml]
---

# 0013 · Pi built-in tool selection via `tools:` field in agent YAML

Date: 2026-04-16
Status: accepted
Related: ADR 0003 (runtime interface — may revisit vocabulary), ADR 0005 (Pi runtime)

## Context

Pi has a set of built-in tools (bash, read, edit, write, grep, glob, etc.).
Not every agent should have every tool. A content-writer agent doesn't
need bash; a code-reviewer doesn't need write access to arbitrary paths.

Zund needs to express per-agent tool selection declaratively.

## Decision

Agent YAML includes a `tools:` array listing the Pi built-in tool names
to enable for that agent.

```yaml
kind: Agent
name: code-reviewer
tools:
  - read
  - grep
  - glob
```

An empty array means no built-in tools (useful for ephemeral probe agents).

The Zund-provided tools (`memory_save`, `memory_search`, `zund_emit_artifact`,
`working_memory_update`, `zund_fleet_status`) are always available, added by
the Pi extension. They are *not* in the `tools:` list — that list is strictly
Pi built-ins.

## Consequences

**Makes easier:**

- Tight per-agent capability control.
- Safe defaults (no bash unless declared).
- Readable YAML — it's obvious what an agent can do.

**Makes harder:**

- The `tools:` vocabulary is Pi-specific. A VM runtime or SSH runtime would
  have a different tool set. Under ADR 0003 this becomes a runtime concern:
  the field may be renamed `capabilities:` and given neutral semantics, with
  each runtime declaring its own vocabulary.
- Skills (separate concept) also provide capabilities. The `tools:` / skills
  distinction must be clear to users.

## Implementation notes

- Parsed in `packages/daemon/src/fleet/schema.ts`.
- Validated in `packages/daemon/src/fleet/validator.ts` — names must match
  known Pi built-ins.
- Passed through to the Pi extension writer which configures Pi's tool
  registration at agent launch.
