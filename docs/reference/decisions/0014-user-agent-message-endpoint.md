---
id: "0014"
title: User-to-agent interaction — message endpoint + SSE stream
date: 2026-04-16
status: accepted
implementation: not-started
supersedes: []
superseded_by: null
related: ["0002", "0008"]
tags: [access, api, streaming, l4]
---

# 0014 · User-to-agent interaction — message endpoint + SSE stream

Date: 2026-04-16
Status: accepted
Related: ADR 0002 (stream protocol), ADR 0008 (dual Bun.serve)

## Context

Users (via CLI or console) need to send messages to running agents and
receive responses. Options considered:

- **Polling** — simple but laggy; bad UX for streaming LLM output.
- **WebSocket** — bidirectional but heavier; browser EventSource is simpler.
- **SSE** — unidirectional server→client, but messages can go
  client→server via separate POST.
- **gRPC streaming** — too much for local single-host.

## Decision

Single HTTP POST endpoint with optional SSE streaming response.

**Request:** `POST /v1/agents/:name/message?stream=true|false`

**Body:** `{ "content": "..." }`

**Response (stream=false):** JSON with the final assistant message.

**Response (stream=true):** SSE with incremental events.

Events (current, Pi-native):
```
event: message_update          # text deltas
event: tool_execution_start    # tool invocation begun
event: tool_execution_end      # tool result available
event: error
event: stream_end
```

Events (canonical, under ADR 0002):
```
event: text-start / text-delta / text-end
event: tool-input-start / tool-input-delta / tool-input-available / tool-output-available
event: artifact-created / memory-updated
event: error / done
```

The transition from Pi-native events to canonical events is the
Event Translator work in L3 (planned).

Fleet-wide events (agent lifecycle, health) use a separate SSE channel
at `GET /v1/events` — a broadcast stream, not per-request.

## Consequences

**Makes easier:**

- Browser-native: `EventSource` just works.
- CLI SSE parsing is 30 lines — see `packages/cli/src/transport/sse.ts`.
- Auto-reconnect on disconnect is a standard EventSource feature.
- One endpoint serves both streaming and non-streaming consumers.

**Makes harder:**

- SSE is unidirectional. Client-initiated follow-up (cancel, interrupt)
  requires a separate HTTP call, not a message on the same channel.
- 30-second keepalives are needed to prevent proxy timeouts.
- Two SSE channels (per-agent message stream + fleet broadcast) means
  clients manage two connections. Acceptable — they have different scopes.

## Implementation notes

- Endpoint at `packages/daemon/src/api/server.ts`.
- Fleet broadcast uses a shared `Set<Controller>` for all subscribed clients.
- Message stream holds the `incus exec` stdout pipe open per active request.
- Console uses `useEventSource()` hook with exponential backoff
  (`packages/console/src/lib/hooks/`).
