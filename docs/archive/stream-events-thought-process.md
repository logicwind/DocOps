# zund://stream/v1 — Thought Process & Open Questions

> **Status:** Pre-ADR brainstorm. Not a decision document. Pick this up when
> ready to finalize the canonical event vocabulary and client contracts.
>
> **Context:** ADR 0002 defines the protocol frame (SSE, `x-zund-stream: v1`
> header, translator seam). ADR 0003 defines the `Runtime` interface. The
> translator exists as an identity passthrough. This document covers the
> *vocabulary* — what events flow over the wire, their shapes, state machines,
> and the design questions that need answers before freezing the contract.
>
> **Frontend:** The console will use [assistant-ui](https://github.com/assistant-ui/assistant-ui)
> for the chat UI. This library has a specific wire protocol expectation
> (ui-message-stream) that directly affects our contract design. See §9.

---

## 0. assistant-ui impact on contract design

The console will use [assistant-ui](https://github.com/assistant-ui/assistant-ui)
for the chat interface. This library has a well-defined wire protocol that
our SSE stream must be compatible with — either directly or via an adapter.

### assistant-ui's wire protocol: ui-message-stream

assistant-ui supports two stream protocols. The default (and recommended)
one is **ui-message-stream**. Its event shapes:

```typescript
// assistant-ui/packages/assistant-stream/.../chunk-types.ts

type UIMessageStreamChunk =
  // Message lifecycle
  | { type: "start"; messageId: string }
  | { type: "text-start"; id: string }
  | { type: "text-delta"; textDelta: string }
  | { type: "text-end" }

  // Reasoning (thinking)
  | { type: "reasoning-start"; id: string }
  | { type: "reasoning-delta"; delta: string }
  | { type: "reasoning-end" }

  // Tool lifecycle
  | { type: "tool-call-start"; id: string; toolCallId: string; toolName: string }
  | { type: "tool-call-delta"; argsText: string }
  | { type: "tool-call-end" }
  | { type: "tool-result"; toolCallId: string; result: unknown; isError?: boolean }

  // Sources and files
  | { type: "source"; source: { sourceType: "url"; id: string; url: string; title?: string } }
  | { type: "file"; file: { mimeType: string; data: string } }

  // Steps and lifecycle
  | { type: "start-step"; messageId?: string }
  | { type: "finish-step"; finishReason: string; usage: { inputTokens: number; outputTokens: number }; isContinued: boolean }
  | { type: "finish"; finishReason: string; usage: { inputTokens: number; outputTokens: number } }
  | { type: "error"; errorText: string }

  // Extensible custom data (this is our escape hatch for Zund-native events)
  | { type: `data-${string}`; id?: string; data: unknown; transient?: boolean }
```

Key observations:

1. **The message/tool lifecycle is nearly identical to ADR 0002's proposed
   vocabulary.** `text-start/delta/end`, `tool-call-start/delta/end`, `finish`,
   `error` — the shape is the same, just different naming in a few places
   (ADR 0002 says `tool-input-start`, assistant-ui says `tool-call-start`).

2. **The `data-*` extension channel is purpose-built for custom events.** Any
   event type starting with `data-` is ignored by assistant-ui's default
   decoder and passed through to an `onData` callback. This is exactly where
   Zund-native events (artifacts, memory, tasks, fleet) should go.

3. **assistant-ui owns the UI rendering loop.** The library manages message
   state, tool call status, streaming text accumulation, step tracking, and
   UI updates. We don't build any of that — we feed it events via the
   `useDataStreamRuntime` hook (or a custom adapter), and it renders.

### Implications for Zund's contract design

**Three options:**

| Option | What Zund emits | Console integration | Coupling |
|--------|-----------------|---------------------|--------|
| A: Zund emits ui-message-stream directly | `UIMessageStreamChunk` types | Zero adaptation. Console uses `useDataStreamRuntime` with `protocol: "ui-message-stream"`. | Tight: Zund's wire format IS assistant-ui's format. If they change, we change. |
| B: Zund owns its vocabulary, provides an adapter | `CanonicalEvent` types, adapter converts to `UIMessageStreamChunk` | One thin adapter layer. Console uses `useDataStreamRuntime` with `protocol: "ui-message-stream"` pointing at an adapter endpoint. | Loose: Zund's contract is independent. If assistant-ui changes, we update the adapter, not the wire protocol. |
| C: Zund emits ui-message-stream + `data-*` extensions | `UIMessageStreamChunk` with `data-artifact-created`, `data-memory-updated`, etc. | Zero adaptation. Zund-native events are `data-*` parts that pass through `onData`. | Medium: message/tool events are coupled, Zund concepts are loose. |

**Recommendation: Option B**, with the vocabulary designed to make the adapter
trivial. This means:

- Zund's canonical vocabulary **closely mirrors** ui-message-stream for
  message and tool lifecycle events (same start/delta/end pattern, similar
  field names, same semantics).
- Zund-native concepts (artifacts, memory, tasks, fleet) use their own
  event types in the canonical vocabulary (e.g., `artifact-created`,
  `task-dispatched`).
- The adapter maps canonical → ui-message-stream one-to-one for
  LLM-related events, and maps Zund-native events to `data-*` parts.
- If we ever drop assistant-ui, our wire format doesn't change.

**The adapter mapping would look like:**

```
zund://stream/v1                    →  ui-message-stream
─────────────────────────────────────────────────────────────
start { messageId }                 →  start { messageId }
text-start { id }                   →  text-start { id }
text-delta { id, delta }            →  text-delta { textDelta: delta }
text-end { id }                     →  text-end { }
reasoning-start { id }              →  reasoning-start { id }
reasoning-delta { id, delta }        →  reasoning-delta { delta }
reasoning-end { id }                →  reasoning-end { }
tool-start { id, toolCallId, name } →  tool-call-start { id, toolCallId, toolName }
tool-delta { id, argsTextDelta }    →  tool-call-delta { argsText }
tool-end { id }                     →  tool-call-end { }
tool-result { toolCallId, result }   →  tool-result { toolCallId, result, isError? }
finish { reason, usage }             →  finish { finishReason, usage }
error { code, message }             →  error { errorText }

artifact-created { ... }            →  data-artifact-created { ... }
memory-updated { ... }              →  data-memory-updated { ... }
task-dispatched { ... }             →  data-task-dispatched { ... }
agent-started { ... }               →  data-agent-started { ... }
```

**Why not Option A (emit ui-message-stream directly):**
- assistant-ui is pre-1.0. Types are prefixed `unstable_`. The protocol may
  change. If we emit their format directly, we absorb their breaking changes
  into our wire protocol, which means our v1 contract isn't in our control.
- Non-assistant-ui consumers (CLI, future SDK) would receive a format
  designed for a specific React library. That's leaky abstraction.

**Why not Option C (ui-message-stream + data-* extensions):**
- Puts us in a half-coupled state: LLM events are tied to assistant-ui,
  Zund events aren't. This makes the contract inconsistent.
  Renaming `tool-input-start` to `tool-call-start` isn't just a rename —
  it's conceding that our vocabulary is defined by a third party.

### What we must match vs. what we must differ

**Match (same semantics, minor naming differences):**
- Start/delta/end streaming pattern for text and reasoning
- Tool call lifecycle: call start → args stream → result
- Step/message tracking: start/finish for steps, message lifecycle
- Error and finish as terminal events
- `id` / `toolCallId` / `messageId` correlation fields

**Differ (Zund-native, map to `data-*`):**
- Artifacts (`artifact-created`, `artifact-creating`)
- Memory (`memory-updated`, `memory-slot-patched`)
- Tasks (`task-queued`, `task-dispatched`, `task-completed`)
- Agent lifecycle (`agent-started`, `agent-stopped`)
- Fleet events (`fleet-apply-started`, `fleet-health`)
- Zund-specific metadata (scope, fleet name, etc.)

### Open questions for assistant-ui integration

- **Adapter location**: Does the adapter live in the console package
  (`packages/console`), or in `@zund/core`? If in core, it's a dependency
  on assistant-ui types. If in console, it's console-specific. Probably
  console — the adapter is a UI concern, not a daemon concern.

- **Custom data handling**: assistant-ui's `data-*` parts go through
  `onData` callback, not through the message rendering pipeline. The
  console will need custom React components to render `data-artifact-created`
  etc. inside the chat thread. How does assistant-ui handle custom rendering
  for data parts? (Answer: it supports `data-*` parts with custom renderers
  via `onData` + custom message components.)

- **Non-streaming responses**: assistant-ui's `useDataStreamRuntime` expects
  an SSE endpoint. Our `POST /v1/agents/:name/message?stream=false` returns
  JSON. Does the console always use streaming? If not, we need a different
  adapter path for the non-streaming case.

- **Multi-step conversations**: assistant-ui supports multi-step (agentic)
  conversations natively. Does the Zund per-request SSE stream map cleanly
  to assistant-ui's thread model, or do we need to maintain a session state
  on the client side?

- **Tool call rendering**: assistant-ui renders tool calls with a built-in
  `MakeDefaultToolCall` component. Zund tool calls may have custom rendering
  (showing artifact previews, memory diffs, etc.). How much of the default
  rendering do we override?

---

## 1. The fundamental tension: Pi-native vs Zund-owned

Right now the SSE stream sends Pi's raw events to clients. The translator is
a no-op. Every client (CLI, console, future SDK) couples to Pi's vocabulary.

| Factor | Keep Pi-native passthrough | Define Zund-owned vocabulary |
|--------|---------------------------|------------------------------|
| **Speed** | Zero work today, ship now | Must design mappings, test edge cases |
| **Coupling** | Every client knows Pi internals | Clients depend only on Zund contract |
| **Multi-runtime** | Second runtime breaks all clients | Translator absorbs the difference |
| **Zund concepts** | Nowhere to put artifact/task/memory events | First-class events for Zund-native things |
| **Iteration** | Free to change (no contract) | Breaking changes require v2 bump |
| **Debuggability** | Raw Pi events are familiar if you know Pi | Translation layer obscures Pi details |

**Question:** When is the cost of migration higher — now (small client surface) or later (multiple clients + runtimes)? What's the client surface today? CLI + console only, or are external consumers expected soon?

---

## 2. Where do the event types live?

Three options for where to put the canonical event types:

| Option | Location | Pros | Cons |
|--------|----------|------|------|
| A | `@zund/core/contracts/events.ts` (current file) | Single source of truth, plugins and clients share | Core package becomes a kitchen sink; wire types aren't plugin contracts |
| B | New `@zund/core/contracts/wire.ts` | Separates daemon↔plugin from daemon↔client | Two event type systems to maintain; CanonicalEvent vs WireEvent |
| C | New `@zund/types` package | Clients (CLI, console) depend on a tiny package; daemon and plugins don't carry wire types | Another package to publish/version |

**Question:** Who is the primary consumer of wire events? Is it always going through an API boundary (SSE), or do internal consumers (like the console server component) need direct access?

**Question:** Should the wire types be the same TypeScript types used in the translator, or should there be a serialization step (translator produces runtime-agnostic TypeScript objects → serialize to wire JSON)?

---

## 3. Event categories and their design questions

### 3.1 Message lifecycle events

The core loop: user sends a message → agent processes → text streams back.

Pi today emits: `message_start`, `message_delta` (text content), `message_stop`,
plus `content_block_start/delta/stop` for structured content.

```
Proposed canonical:
  start          { messageId }
  text-start     { id }
  text-delta     { id, delta }
  text-end       { id }
```

**Questions:**

- **Message ID**: Who generates it — the daemon on inbound, or does the client
  send it? If client-generated, what about idempotency? If daemon-generated,
  how does the client correlate request→response?
- **Multiple text blocks**: A single agent response can have multiple text
  blocks (e.g., thinking + response). Do we represent each as a separate
  `text-start/delta/end` sequence, or is there a `content-block` concept?
- **Streaming vs. non-streaming**: The `POST /v1/agents/:name/message` endpoint
  supports `stream=false`. Does `start` event still fire, or is the response
  just a JSON object? Should the canonical vocabulary only apply to SSE?

- **Thinking/reasoning**: Some models emit reasoning tokens. Do we surface
  these as a distinct event type (`reasoning-start/delta/end`) or fold them
  into `text-start/delta/end` with a `role: "thinking"` field?

### 3.2 Tool lifecycle events

Pi emits: `tool_execution_start`, `tool_execution_end`.

```
Proposed canonical:
  tool-input-start      { toolCallId, toolName }
  tool-input-delta      { toolCallId, inputTextDelta }
  tool-input-available  { toolCallId, input }
  tool-output-available { toolCallId, output }
```

**Questions:**

- **Tool ID**: Pi generates tool call IDs. Do we pass those through as-is,
  or does Zund generate its own? If a second runtime uses different ID
  shapes, the contract needs to be agnostic (string? UUID?).

- **Streaming input**: `tool-input-delta` streams the tool's input arguments
  as the model generates them. This is useful for UI (show the tool call
  forming) but not all runtimes support it. Should it be optional in the
  contract? How does the client know if deltas are coming?

- **Streaming output**: Some tools produce output incrementally (a long-running
  bash command). Do we need `tool-output-delta`? Or is
  `tool-output-available` sufficient with the convention that it fires once
  per tool call? What about tools that produce multiple artifacts?

- **Tool errors**: How do tool errors surface? A separate `tool-error`
  event? An `error` field on `tool-output-available`? Both? What about
  partial success (tool ran but produced unexpected output)?

- **Tool list**: Should the client know what tools an agent has before
  a tool call starts? If so, what event carries that? A `tools-available`
  event at session start? Or is that a separate API call, not a stream event?

### 3.3 Artifact events — Zund-native

This is where it gets Zund-specific. Artifacts are content-addressed blobs
created by agents (files, images, audio, etc.).

```
Proposed canonical:
  artifact-created      { artifactId, url, kind, mimeType, label }
```

**Questions:**

- **Creation lifecycle**: How does the client know an artifact is being
  *created* (in progress) vs. *created* (done)? Options:
  - Single `artifact-created` fires when the blob is fully stored and URL
    is available. Simple, but the client sees nothing during creation.
  - Two events: `artifact-creating` (in progress) → `artifact-created` (done).
    Client can show a spinner. But what if creation fails?
  - Three events: `artifact-creating` → `artifact-created` →
    `artifact-failed`. Most complete, most complex.
  - Do we need progress at all? For small text snippets, creation is instant.
    For large binaries, it could take seconds. Should there be a progress
    percentage, or just started/done?

- **URL availability**: When does the URL become available? Before the blob
  is fully written to disk? After? If the client gets an `artifact-created`
  event with a URL but the blob isn't readable yet, what happens?

- **Agent creating artifact for another agent**: In a fleet, agent A might
  produce an artifact that agent B consumes. Does agent B get an
  `artifact-created` event? How? Through the fleet broadcast stream
  (`/v1/events`), not the per-agent stream? Or through the dispatcher?

- **Content-addressed implications**: Two agents producing identical content
  get the same artifact ID. Is that a problem for the client? Does the
  client need to know which agent created it (there's an `agent` field in
  `ArtifactMeta`, but that's in the metadata, not the SSE event)?

### 3.4 Memory events — Zund-native

```
Proposed canonical:
  memory-updated        { agent, scope, kind }
```

**Questions:**

- **Granularity**: `memory-updated` fires after any change (fact saved,
  working memory patched, facts pruned). Is this enough for the client?
  Or do we need `fact-saved`, `memory-patched`, `facts-pruned` as separate
  events?

- **Content in events or not**: Should `memory-updated` include the new
  content, or just a signal that something changed (and the client fetches
  via API)? Including content means larger events and potential staleness
  if the client processes events out of order. Not including content means
  the client must make an API call to see what changed.

- **Scope visibility**: An agent's private memory (`scope: "agent:writer"`)
  vs. team memory vs. fleet memory. Does the event stream respect scope?
  If agent A saves a fact to its private scope, does agent B see a
  `memory-updated` event? Probably not — but the stream is per-agent,
  so this might be natural (each agent only sees their own events). What
  about the console viewing multiple agents?

- **Working memory as a living document**: Working memory can be patched
  (H2 slot replace/append/delete). Should there be a `memory-slot-patched`
  event that includes the diff, or is `memory-updated` sufficient?

### 3.5 Task / dispatch events — Zund-native (planned)

The task queue / dispatcher doesn't exist yet, but ADR 0002 anticipates it.

```
Proposed canonical:
  task-queued           { taskId, source, prompt }
  task-dispatched       { taskId, agent, reasoning }
  task-pending          { taskId, reason }
  task-completed        { taskId, result }
```

**Questions:**

- **State machine**: What are the valid states and transitions?

  ```
  queued → dispatched → running → completed
                    ↘ pending      ↘ failed
  ```

  Is `pending` a state (suspended, waiting for something) or an event
  (notification that something is delayed)? What are the valid transitions?

  Can a task go: queued → completed (agent handles instantly)?
  Can a task go: dispatched → queued (re-queued)?
  Can a task go: pending → failed (timeout)?

- **"Planned but not executed"**: The user mentioned: "let's say, task is
  planned for future. Like it's not going to get executed. So what state
  does it go to?"

  This could be:
  - `task-planned` — a new state meaning "will be executed at a later time
    / when conditions are met" (different from `pending` which implies
    "was attempting but blocked").
  - `task-queued` with a `scheduledAt` timestamp — the queue holds it until
    the time comes. The client sees `task-queued` and knows it's deferred.
  - `task-pending` with `reason: "scheduled"` — reuse the existing pending
    state with a reason field.

  **Key question:** Is "planned but not executing yet" fundamentally
  different from "pending/blocked"? A scheduled task hasn't started;
  a pending task started but hit a wall. Do we need distinct states?

- **Task source**: Who can create tasks? The user (via API), the dispatcher
  (auto-routing), or agents (delegating to each other)? Does the event
  need a `source` field that distinguishes these?

- **Task result**: What's in `result`? Just a summary string? The agent's
  final message? A reference to artifacts produced? A URL to fetch details?

- **Cancellation**: Is there a `task-cancelled` event? What about
  `task-timed-out`? Or are these all `task-completed` with a `status` field?

- **Task visibility**: Does every agent see every task event, or only
  the dispatcher and the assigned agent? Fleet broadcast stream vs.
  per-agent stream?

### 3.6 Agent lifecycle events

```
What exists in Pi:
  (implicit — container starts, agent responds, container stops)

What Zund needs:
  agent-started        { agentName, runtime }
  agent-stopped        { agentName, reason }  // reason: "idle" | "error" | "terminated"
  agent-error          { agentName, code, message, recoverable }
```

**Questions:**

- **Where does agent lifecycle live?** Per-agent message stream or fleet
  broadcast stream (`/v1/events`)? ADR 0014 says lifecycle events go on the
  fleet broadcast. But what about an agent's own start/stop — should that
  also appear on its per-agent stream so a client watching one agent gets
  a complete timeline?

- **Crash recovery**: When an agent crashes and `zundd` restarts it, what
  events fire? `agent-stopped { reason: "error" }` then `agent-started`?
  Or `agent-restarting`? How does the client know it's a restart vs. a
  new session?

- **Ephemeral vs. persistent**: ADR 0018 distinguishes persistent agents
  (long-lived) from ephemeral (created per-task, destroyed after). Does
  the lifecycle event carry the agent's lifecycle type? Does the client
  need to know?

### 3.7 Fleet-level events

Beyond individual agents, the fleet as a whole has events:

```
What might be needed:
  fleet-apply-started    { fleet, agentsCreated, agentsUpdated, agentsDeleted }
  fleet-apply-completed  { fleet, result }
  fleet-health           { agents: [{ name, status }] }
```

**Questions:**

- **Apply events**: When `zund apply` runs, should the stream carry
  reconciliation events? "Agent writer: creating container", "Agent writer:
  injecting secrets", etc.? Or is that a separate API response?

- **Health heartbeat**: Does the fleet broadcast stream send periodic health
  pings? Or only events on state change?

- **Fleet size**: In a fleet with 50 agents, does every client get 50x
  `agent-started` events? Is there a scalable subscription model (watch
  specific agents only)?

---

## 4. Event envelope and metadata

Regardless of event type, every event might need common fields:

```typescript
interface CanonicalEvent {
  type: string;                    // discriminant
  id: string;                      // unique event ID (for idempotency / dedup)
  timestamp: number;               // unix ms (when the event occurred, not sent)
  agentName?: string;              // originating agent (when applicable)
  // ...type-specific fields
}
```

**Questions:**

- **Event ID**: Do we need unique IDs on every event? If so, who generates
  them — the daemon (simple) or the runtime (more distributed)? What about
  replay/dedup — does the client need to handle duplicate events?

- **Timestamp**: Daemon-generated (when event was translated) or
  runtime-generated (when the event actually happened)? For Pi, the runtime
  produces events with timestamps; for other runtimes, who knows.

- **Correlation**: How does a client correlate `text-start`, `text-delta`,
  `text-end` to a single response? A `messageId` on all events in the
  response? A session-level `requestId`? Or is SSE ordering sufficient
  (all events between `start` and `done` belong to one response)?

- **Ordering guarantee**: Are events guaranteed to be in order on SSE?
  If the translator processes events from multiple sources (runtime events
  + artifact events + memory events), can they interleave? Is that desired?

---

## 5. The translator design

The translator sits between runtimes and the wire:

```
Pi runtime ──→ RuntimeEvent ──→ translateEvent("pi", event) ──→ CanonicalEvent ──→ SSE
Hermes ──────→ RuntimeEvent ──→ translateEvent("hermes", event) ──→ CanonicalEvent ──→ SSE
Zund internals (artifact store, memory, dispatcher) ──→ CanonicalEvent ──→ SSE
```

**Questions:**

- **Non-runtime events**: Artifacts, memory, and task events aren't produced
  by a runtime. They're produced by the daemon (L2/L3). Do they go through
  the translator, or do they bypass it and emit CanonicalEvents directly?
  If the translator is the single funnel, it needs to accept internal
  events too. If internal events bypass it, we have two paths to the wire.

- **Translator registration**: Is `translateEvent(runtimeName, event)` the
  right signature? Should each runtime register a mapper function at startup?
  Should it be a map of `{ runtimeName: mapperFn }`?

- **Lossy translation**: What happens when a runtime emits an event that
  has no canonical equivalent? Options:
  - Drop it (client never sees it — may lose useful info)
  - Pass it through as `{ type: "raw", runtime: "pi", data: ... }` (leaks
    runtime details, but preserves information)
  - Warn and drop (log a warning, useful during development)

- **Events from multiple agents on one stream**: If a user watches the fleet
  broadcast (`/v1/events`), events from multiple agents arrive on one SSE
  connection. Does the `agentName` field on every event suffice, or should
  there be a `source` envelope?

---

## 6. Versioning and compatibility

ADR 0002 specifies: version negotiation via `x-zund-stream: v1` header.

**Questions:**

- **Additive changes**: Adding a new event type to v1 — is this a breaking
  change? Clients that don't know about the new type should ignore it.
  Do we document this as a rule ("unknown event types must be ignored")?

- **Field additions**: Adding a field to an existing event type — is this
  breaking? If clients use strict parsing (TypeScript discriminated unions),
  extra fields may cause errors. Should we mandate that clients must ignore
  unknown fields?

- **v2 trigger**: What would force a v2? Renaming an event type? Removing a
  field? Changing the semantics of an existing field?

- **Client library**: Should we ship a TypeScript client library that handles
  event parsing, dedup, and reconnection? Or is the contract enough and
  every consumer writes their own?

---

## 7. What the console needs today

The console renders: agent messages, tool calls (with input/output), and
artifact links. It does NOT currently render: task progress, memory changes,
fleet health.

**Question:** Design for today's consumers only, or design for the full
vocabulary and implement what we need? Recommendation: define the full
vocabulary now but only implement translations for what the console needs
today. Task and fleet events can be no-ops until those features are built.

---

## 8. Decision checklist (pick up here)

When you're ready to finalize, work through these in order:

- [ ] **assistant-ui strategy**: Decide between Option A, B, or C (§0).
  Recommended: Option B (Zund-owned vocabulary, thin adapter to ui-message-stream).
- [ ] **Adapter location**: Decide where the Zund→assistant-ui adapter lives
  (console package vs. core). Probably console.
- [ ] **Wire types vs. plugin contracts**: Decide where the SSE event types
  live. (Option A, B, or C from §2.)
- [ ] **Envelope fields**: Decide on common fields (id, timestamp,
  agentName, correlation). (§4)
- [ ] **Message lifecycle**: Define start/text-start/text-delta/text-end
  shapes. Must map trivially to assistant-ui's text-start/text-delta/text-end.
  Resolve message ID, multi-block, thinking/reasoning. (§3.1)
- [ ] **Tool lifecycle**: Define tool-start/tool-delta/tool-end/tool-result
  shapes. Must map trivially to assistant-ui's tool-call-start/delta/end
  + tool-result. Resolve streaming output, errors, tool list. (§3.2)
- [ ] **Zund-native events — artifacts**: Decide creation lifecycle
  (in-progress vs. created), URL availability. Must map to `data-artifact-*`
  in the adapter. (§3.3)
- [ ] **Zund-native events — memory**: Decide granularity and content
  inclusion. Must map to `data-memory-*` in the adapter. (§3.4)
- [ ] **Zund-native events — tasks**: Define state machine, "planned"
  state, cancellation. Must map to `data-task-*` in the adapter. (§3.5)
- [ ] **Lifecycle events**: Define agent-started/stopped/error. Decide
  per-agent vs. fleet broadcast. Must map to `data-agent-*` in the adapter. (§3.6)
- [ ] **Fleet events**: Decide what goes on `/v1/events` broadcast.
  Must map to `data-fleet-*` in the adapter. (§3.7)
- [ ] **Control events**: Define error and done shapes. These cap every
  stream and must map to assistant-ui's `finish`/`error`. (§3.6)
- [ ] **Translator design**: Decide runtime mapper registration and
  non-runtime event path. (§5)
- [ ] **Compatibility rules**: Decide what constitutes a breaking change
  and document "ignore unknown" rules. (§6)
- [ ] **Implement**: Update `contracts/events.ts` with discriminated union.
  Write Pi mapper. Write console adapter. Update translator. Ship v1.