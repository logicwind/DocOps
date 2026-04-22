---
id: "0004"
title: "Incus as substrate; ephemeral agents via `incus copy --ephemeral`"
date: 2026-04-16
status: accepted
implementation: in-progress
supersedes: []
superseded_by: null
related: ["0006", "0007"]
tags: [substrate, incus, ephemeral, l1]
---

# 0004 · Incus as substrate; ephemeral agents via `incus copy --ephemeral`

Date: 2026-04-16
Status: accepted
Evidence: POC 01 (`experiments/01-incus-from-ts/`), POC 06 (`experiments/06-ephemeral-clone/`)

## Context

Zund needs a container runtime for agent execution with:

- Fast cold start (users expect <5s to message a new agent)
- Fast ephemeral spawn (task-driven agents need sub-second trigger-to-ready)
- Linux-native (no VM overhead on dev machines running Linux)
- Scriptable API (no shelling out to CLI)
- Single-host deployment footprint (dev laptop or small Linux host)

Candidates considered: Docker, Kubernetes, Firecracker, Incus.

## Decision

Use Incus as the container substrate, with `incus copy --ephemeral` from a
template container for ephemeral agent spawns.

**Why Incus over alternatives:**

- Docker: no native ephemeral clone; spinning up from image is 2–5× slower.
- Kubernetes: too heavy for single-host; operationally inappropriate.
- Firecracker: VM-grade isolation we don't need; no mature TypeScript client.
- Incus: LXD fork with mature API, HTTP over Unix socket, native support
  for `copy --ephemeral` that auto-deletes on stop.

**Why ephemeral via copy from template:**

POC 06 measured:
- Clone time: ~105ms average
- Start time: ~116ms average
- Total trigger-to-ready: **228ms average**
- 3 concurrent clones: 447ms wall clock

Well under the <5s target.

## Consequences

**Makes easier:**

- Ephemeral agents (task-driven, cron-triggered) become practical — fast
  enough for interactive use.
- Single-host deployment — Incus runs fine on a laptop-class Linux host.
- State isolation — each agent gets its own container with its own filesystem
  namespace.

**Makes harder:**

- Linux-only substrate. macOS dev requires a Linux VM (OrbStack, Lima).
- Operational dependency on Incus being installed and configured on the host.
- Incus API is LXD-compatible but less well-known than Docker.

## Implementation notes

### Substrate layer (complete)

- HTTP client at `apps/daemon/src/incus/client.ts` uses Bun native fetch
  over Unix socket (ADR 0006).
- Base image is `zund/base:<series>`, produced by `zund image build`
  (ADR 0007).
- Long-lived agent launch runs through
  `@zund/plugin-runtime-pi`'s `launchAgent`, called from
  `apps/daemon/src/agents/launcher.ts::launchLongLivedAgent`.

### Ephemeral wiring (in progress — this ADR)

- **Incus primitives (done).** `copyContainer` + `getContainerState` in
  `apps/daemon/src/incus/containers.ts`; `cloneEphemeral` convenience
  helper in `apps/daemon/src/incus/ephemeral.ts`.
- **Template provisioner (done).** `ensureTemplateContainer` in
  `apps/daemon/src/incus/ensure-template-container.ts` guarantees a
  stopped template container exists on boot; `deriveTemplateContainerName`
  keeps the `zund-template-<series>` naming convention in one place.
  Non-fatal pre-flight in `api/server.ts`.
- **Facade extension (done).** `ZundIncusFacade` gains `copyContainer`,
  `startContainer`, `stopContainer` so plugins clone without importing
  `IncusClient`.
- **Plugin launch (done).**
  `@zund/plugin-runtime-pi/launcher::launchEphemeralAgent` composes
  copy + start + optional mounts + skill mounts + env vars + Pi config +
  RPC session start. Sibling to `launchAgent`; shares RPC transport.
  Stop via `stopEphemeralAgent` — Incus auto-deletes the clone.
- **Daemon wrapper (done).**
  `apps/daemon/src/agents/launcher.ts::launchEphemeralAgent` maps
  `RuntimeState` → plugin call and writes the `zund-fleet` extension so
  memory/artifact/fleet bridges work identically to long-lived agents.
  No `state.agents` registration — the caller owns the handle for the
  task's duration.

### Execution model

v1 ships **RPC-mode only** for both UI/terminal and queue use cases. The
daemon holds the live RPC session for the task; result is returned
in-band, not via HTTP callback. Retry policy lives with the caller (UI
re-issues on session drop; task queue re-enqueues on promise reject).

Exec-mode (Pi-to-daemon HTTP callback with idempotent `(task_id,
attempt_id)` dedup) is deferred — will land if streaming-result or
durable-attempt requirements show up in the ADR 0023 task queue
implementation.

### Remaining work

- **ADR 0023 task queue caller** treats `launchEphemeralAgent` as a
  blocking async call. Lives with the task queue slice, not here.
- **Integration test.** `test/integration/ephemeral-agent.test.ts`
  against a real Incus + prebuilt template container. Asserts <5s
  trigger-to-ready (POC 06 baseline: 228ms).
- **Boot-time reconcile for orphan ephemerals.** On restart, sweep any
  `ephemeral-*` containers whose task rows are terminal — those are
  leaks from a prior daemon crash. Lands with the task queue so there's
  a task row to cross-reference.

### Mount strategy

Ephemeral spawns run **sealed by default** (no host-path bind mounts). For
workflow/sequential chains where agent B consumes agent A's output on
disk, callers pass `mounts: Array<{ hostPath, containerPath, deviceName }>`
via `EphemeralLaunchOptions`. Orchestrator owns host-path lifecycle —
create before spawn, clean up after the chain terminates. The
container's `ephemeral: true` flag handles the container cleanup; the
host-side cleanup is the orchestrator's job.
