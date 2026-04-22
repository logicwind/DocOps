# Runtime Protocol — `zund://stream/v1`

The canonical wire format for events flowing from L3 (Orchestration) to
L4 (Access). See ADR 0002 for the decision rationale, ADR 0003 for the
runtime interface context.

**Status:** proposed. Current production code emits Pi-native events
verbatim. This doc describes the target vocabulary and the translator
layer that converts runtime-native events to it.

---

## Transport

SSE over HTTP. One event per `data:` line, JSON-encoded. Keepalive comment
every 30 seconds.

```
event: text-delta
data: {"id":"msg_01","delta":"Hello"}

: keepalive

event: done
data: {"reason":"stop"}
```

Response header: `x-zund-stream: v1` — lets clients version-check.

Two SSE channels use this vocabulary:

- `GET /v1/events` — fleet broadcast (agent lifecycle, health, task events)
- `POST /v1/agents/:name/message?stream=true` — per-message response stream

---

## Event vocabulary (v1)

### Message lifecycle

| Event | Payload | Meaning |
|-------|---------|---------|
| `start` | `{ messageId }` | A new assistant message begins |
| `text-start` | `{ id }` | A text segment begins |
| `text-delta` | `{ id, delta }` | Text token(s) appended |
| `text-end` | `{ id }` | A text segment is complete |

### Tool lifecycle

| Event | Payload | Meaning |
|-------|---------|---------|
| `tool-input-start` | `{ toolCallId, toolName }` | Tool call initiated |
| `tool-input-delta` | `{ toolCallId, inputTextDelta }` | Streaming tool input (if supported) |
| `tool-input-available` | `{ toolCallId, input }` | Complete tool input ready |
| `tool-output-available` | `{ toolCallId, output }` | Tool result ready |

### Zund-native

| Event | Payload | Meaning |
|-------|---------|---------|
| `artifact-created` | `{ artifactId, url, kind, mimeType, label, size }` | Artifact emitted (ADR 0011) |
| `memory-updated` | `{ agent, scope, kind }` | Fact or working memory updated |

### Task queue (L3 planned)

| Event | Payload | Meaning |
|-------|---------|---------|
| `task-queued` | `{ taskId, source, prompt }` | Task entered queue |
| `task-dispatched` | `{ taskId, agent, reasoning }` | Dispatcher picked an agent |
| `task-pending` | `{ taskId, reason }` | No suitable agent; parked |
| `task-completed` | `{ taskId, result }` | Task finished |

### Fleet lifecycle (broadcast channel)

| Event | Payload | Meaning |
|-------|---------|---------|
| `agent.created` | `{ agent }` | Agent added |
| `agent.destroyed` | `{ agent }` | Agent removed |
| `agent.updated` | `{ agent, fields }` | Agent resource changed |
| `agent.health` | `{ agent, status }` | Health poll result |
| `fleet.deleted` | `{ }` | Entire fleet destroyed |

### Lifecycle

| Event | Payload | Meaning |
|-------|---------|---------|
| `error` | `{ code, message, recoverable }` | Non-fatal or fatal error |
| `done` | `{ reason }` | Stream terminated normally |

---

## Translator architecture

Runtime emits native events → L3 translator maps to canonical → L4 writes
SSE.

```
┌──────────────┐    native events     ┌──────────────┐    canonical    ┌────────┐
│ Pi runtime   │ ────JSONL───────────▶│ L3           │ ───────────────▶│ L4 SSE │
│ (VM, SSH…)   │    runtime-specific  │ Translator   │ zund://stream/v1│        │
└──────────────┘                      └──────────────┘                 └────────┘
```

Each runtime registers a translator function:

```typescript
interface RuntimeTranslator {
  readonly runtime: string;
  translate(event: RuntimeEvent): CanonicalEvent | CanonicalEvent[] | null;
}
```

Returning `null` drops the event (runtime noise not meant for clients).
Returning an array lets one native event expand into multiple canonical
events (e.g., a Pi `message_update` containing text + tool calls may
expand into `text-delta` + `tool-input-start`).

---

## Pi → canonical mapping (reference)

| Pi event | Canonical events |
|----------|------------------|
| `message_update` (text portion) | `text-delta` |
| `tool_execution_start` | `tool-input-start`, `tool-input-available` |
| `tool_execution_end` (artifact in return) | `tool-output-available`, `artifact-created` |
| `tool_execution_end` (memory tool) | `tool-output-available`, `memory-updated` |
| `agent_end` | `done` |
| `error` | `error` |

This table is the authoritative mapping. It lives in
`apps/daemon/src/stream/runtimes/pi.ts` when implemented.

---

## Versioning

- **Additive changes** (new event types, new optional fields) stay in v1.
- **Breaking changes** (removed events, renamed fields, shape changes) bump
  to v2. Both versions are supported for one release cycle.
- Clients negotiate via the `x-zund-stream` response header.

---

## AI SDK compatibility (non-goal, optional)

The event vocabulary intentionally mirrors the shape of Vercel AI SDK's
Data Stream Protocol. This is not a commitment — Zund owns the namespace
and has first-class events AI SDK lacks (artifacts, memory, tasks).

A future adapter could expose the Zund stream as an AI SDK-compatible
endpoint to let React `useChat` consume it directly. That adapter is one
level of indirection on top of this protocol, not the protocol itself.

---

## Implementation status

| Component | Location | Status |
|-----------|----------|--------|
| Canonical event types | `apps/daemon/src/stream/events.ts` | not yet created |
| Translator framework | `apps/daemon/src/stream/translator.ts` | not yet created |
| Pi translator | `apps/daemon/src/stream/runtimes/pi.ts` | not yet created |
| SSE writer | `apps/daemon/src/api/server.ts` (existing) | emits Pi-native; to be replaced |

See `roadmap/next.md` for the implementation ordering.
