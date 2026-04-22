---
id: "0009"
title: "Session storage — host-mounted directory with `shift=true`"
date: 2026-04-16
status: accepted
implementation: done
supersedes: []
superseded_by: null
related: ["0003", "0005", "0015"]
tags: [state, sessions, runtime, l2]
---

# 0009 · Session storage — host-mounted directory with `shift=true`

Date: 2026-04-16
Status: accepted
Evidence: POC 02 (`experiments/02-host-mounted-sessions/`)
Related: ADR 0003 (runtime interface ownership)

## Context

Agent conversations must survive container restarts. Pi writes sessions as
JSONL files. Two options:

- **Inside the container**: sessions die when the container is destroyed
  (ephemeral agents) or get orphaned when the container is recreated.
- **On the host, mounted in**: sessions outlive the container. Requires
  handling uid/gid mapping because the container runs as an unprivileged
  user.

## Decision

Store sessions on the host at `~/.zund/data/sessions/<agent>/` and mount
the directory into the container using Incus device config with
`shift=true`, which transparently remaps uid/gid between host and
container.

POC 02 validated:
- Sessions survive container destroy + recreate.
- Pi reads and writes without permission errors.
- No manual uid remapping needed.

## Consequences

**Makes easier:**

- Agent history persists across restarts, updates, and re-apply.
- Daemon can read sessions directly from host disk (no need to go through
  the container) — used by `SessionIndexer`.
- Simple backup story: tar the sessions dir.

**Makes harder:**

- Requires unprivileged Incus (so `shift=true` works). Root containers do
  not need it.
- Session format is Pi-specific JSONL. Coupled to the runtime — a different
  runtime will materialize sessions differently.
- Under ADR 0003 (runtime interface), session storage becomes owned by the
  Runtime, not the daemon. This ADR captures the current Pi-runtime
  default; the interface-level ADR is 0015.

## Implementation notes

- Device definition in `apps/daemon/src/incus/devices.ts`.
- Sessions indexed in `sessions.db` by `@zund/plugin-runtime-pi` at
  `packages/plugins/runtime-pi/src/sessions/indexer.ts` — bundled with
  the runtime per ADR 0015 (the JSONL format is Pi-specific).
- Ephemeral agents (ADR 0004) still get sessions unless explicitly disabled.
