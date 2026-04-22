---
id: "0008"
title: "Two `Bun.serve` instances (Unix socket + TCP)"
date: 2026-04-16
status: accepted
implementation: done
supersedes: []
superseded_by: null
related: ["0014"]
tags: [access, transport, l4]
---

# 0008 · Two `Bun.serve` instances (Unix socket + TCP)

Date: 2026-04-16
Status: accepted
Evidence: POC 07 (`experiments/07-bun-serve-dual/`)

## Context

Zund has two transport requirements:

- **CLI (local):** must be available without network exposure; clean auth
  story is "if you can access the Unix socket, you're authorized."
- **Console and external clients:** need TCP for browser reachability and
  for cross-host usage.

Routing the same HTTP surface over both transports without duplicating the
router is the ask.

## Decision

Run two `Bun.serve` instances in the same process, sharing the same fetch
handler and the same in-process state via closure.

```typescript
const app = createApp(state);

const unixServer = Bun.serve({
  unix: "/var/run/zundd.sock",
  fetch: app.fetch,
});

const tcpServer = Bun.serve({
  port: 4000,
  fetch: app.fetch,
});
```

Both hit identical routes. State (running agents, SSE subscribers, etc.)
is shared because it's captured by the handler closure.

POC 07 validated that shared state and SSE broadcast work correctly across
both listeners.

## Consequences

**Makes easier:**

- One router, one state model, two transports.
- CLI auth = socket file permissions. No token plumbing needed for local use.
- Console and external integrations get a normal HTTP surface on TCP.

**Makes harder:**

- Security distinction between transports must be enforced at the router
  level when needed (e.g., admin-only routes). Currently all routes are
  available on both.
- Two listeners means two failure modes to handle on startup.
