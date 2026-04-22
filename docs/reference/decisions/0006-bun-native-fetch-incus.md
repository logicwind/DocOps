---
id: "0006"
title: Bun native fetch over Unix socket for Incus
date: 2026-04-16
status: accepted
implementation: done
supersedes: []
superseded_by: null
related: ["0004"]
tags: [runtime, incus, http, l1]
---

# 0006 · Bun native fetch over Unix socket for Incus

Date: 2026-04-16
Status: accepted
Evidence: POC 01 (`experiments/01-incus-from-ts/`)

## Context

Incus exposes its API over a Unix socket (`/var/lib/incus/unix.socket`). The
daemon needs an HTTP client that can talk to it.

Standard Node approaches use `undici` or `node-fetch` with custom dispatchers
to route over Unix sockets. Those add dependencies and complexity.

## Decision

Use Bun's native `fetch()` with `{ unix: socketPath }` option. No extra
dependencies.

```typescript
const res = await fetch("http://localhost/1.0/containers", {
  unix: "/var/lib/incus/unix.socket",
});
```

## Consequences

**Makes easier:**

- Zero additional HTTP dependencies.
- Same fetch API across the codebase (daemon HTTP, CLI HTTP, Incus HTTP).
- Bun runtime is already a hard requirement; no new assumption.

**Makes harder:**

- Locks the daemon to Bun. Node compatibility would require adding back
  undici or equivalent. Acceptable — Bun is chosen for the runtime anyway.
