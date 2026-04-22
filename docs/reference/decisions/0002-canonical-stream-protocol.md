---
id: "0002"
title: Canonical stream protocol (zund://stream/v1)
date: 2026-04-16
status: superseded
implementation: n/a
supersedes: []
superseded_by: "0022"
related: ["0003", "0014", "0020", "0022"]
tags: [protocol, streaming, events, l3, l4]
---

# 0002 · Canonical stream protocol (zund://stream/v1)

Date: 2026-04-16
Status: **superseded by [ADR 0022](0022-stream-protocol-aisdk-uimessage-with-zund-extensions.md)** (2026-04-17)
Related: ADR 0003 (runtime interface), ADR 0014 (message endpoint), ADR 0020 (plugin architecture), ADR 0022 (supersedes this)

> **Superseded.** ADR 0022 revises the vocabulary decision: the wire
> adopts AI SDK's `UIMessage` stream as its base (no parallel Zund-owned
> canonical vocabulary), and adds Zund-native concepts as
> `data-z:<domain>:<event>` parts. The protocol name `zund://stream/v1`
> and the `x-zund-stream: v1` header are retained. The translator seam
> from this ADR becomes an optional per-runtime transform hook (identity
> by default) rather than a mandatory layer. See ADR 0022 for the
> current contract.

## Context

The current agent message stream pipes Pi's native JSONL events verbatim over
SSE to clients:

```
data: {"type":"message_update", ...}
data: {"type":"tool_execution_start", ...}
data: {"type":"tool_execution_end", ...}
data: {"type":"agent_end", ...}
```

The event shape is defined by the Pi runtime, not by Zund. The `RpcEvent`
type in `packages/daemon/src/pi/rpc.ts` is `{ type: string; [key: string]: unknown }`
— a completely open bag. Any future runtime (VM, SSH, alternative LLM harness)
would emit a different vocabulary, and every client (CLI, console, external
SDK consumer) would have to handle both.

Three forces push toward a fixed Zund-owned protocol:

1. **Runtime abstraction (ADR 0003)** requires a neutral wire format so
   clients don't need per-runtime branching.
2. **Task queue + dispatcher (planned L3 work)** will emit task events
   (`task-dispatched`, `task-assigned`, `task-pending`) that should flow
   through the same stream as agent events, giving a unified timeline.
3. **Ecosystem compatibility** — AI SDK's Data Stream Protocol has an almost
   identical shape and is becoming a de facto standard. Aligning the
   vocabulary preserves the option to offer AI SDK compatibility as a view
   later, without taking on the SDK as a dependency today.

## Decision

Define `zund://stream/v1` — a Zund-owned canonical SSE event vocabulary.
Runtimes emit their native events; L3 (Event Translator) converts to the
canonical vocabulary before L4 sends SSE to clients.

Event types (non-exhaustive, frozen within v1):

```
# Message lifecycle
start                 { messageId }
text-start            { id }
text-delta            { id, delta }
text-end              { id }

# Tool lifecycle
tool-input-start      { toolCallId, toolName }
tool-input-delta      { toolCallId, inputTextDelta }
tool-input-available  { toolCallId, input }
tool-output-available { toolCallId, output }

# Zund-native
artifact-created      { artifactId, url, kind, mimeType, label }
memory-updated        { agent, scope, kind }

# Task queue (L3, planned)
task-queued           { taskId, source, prompt }
task-dispatched       { taskId, agent, reasoning }
task-pending          { taskId, reason }
task-completed        { taskId, result }

# Lifecycle
error                 { code, message, recoverable }
done                  { reason }
```

Version negotiation via response header: `x-zund-stream: v1`. Future
versions add new event types; breaking renames bump the version.

The vocabulary mirrors the *shape* of AI SDK's Data Stream Protocol
(start → delta → end lifecycle, typed events, tool input/output split) but
**keeps the namespace in Zund's control**. Zund-native concepts (artifacts,
memory, tasks) have first-class events; they do not exist in AI SDK.

## Consequences

**Makes easier:**

- Swap Pi for another runtime without breaking CLI or console.
- Task queue and agent output share one timeline — "what's happening" view
  is uniform.
- AI SDK compatibility becomes a translation adapter over our stream, not a
  hard dependency.
- Event logging, replay, and audit are easier against a fixed schema.

**Makes harder:**

- Adds a translation layer in L3. Pi emits JSONL, translator converts to
  canonical events. Maintenance cost is one mapping table per runtime.
- Requires versioning discipline. Once v1 is shipped, breaking changes
  require v2.
- Some runtimes may have events that don't map cleanly; those either get
  translated lossily or require v1 to add event types.

## Designs for ADR 0020

### Structural seam (implemented now)

The canonical event type (`CanonicalEvent`) and the translator seam
live in `@zund/core/contracts/events.ts` and `stream/translator.ts`.
The translator is currently an identity passthrough — Pi's native events
flow through unchanged. This creates the architectural seam without
changing the wire format.

When the canonical vocabulary is formally designed:
- `CanonicalEvent` gets a discriminated union with typed variants
- `translateEvent()` dispatches to per-runtime mappers
- `server.ts` already goes through the translator, so no wiring change

The `x-zund-stream: v1` header is now sent on every SSE response.
Terminal stream events (`stream_end`, `stream_error`) are unchanged —
they will be renamed to the canonical `done`/`error` shapes when
the vocabulary is formally designed and the frontend is updated in lockstep.

### Vocabulary design (deferred)

The full event type catalog (`text-delta`, `tool-input-start`,
`artifact-created`, `task-queued`, etc.) is deferred to a follow-up.
Pi's native event types serve as the working v1 vocabulary until we
have real multi-runtime data to inform the canonical mapping.
