---
id: "0022"
title: "Stream protocol v1 — AI SDK UIMessage base with data-z:* Zund extensions"
date: 2026-04-17
status: accepted
implementation: done
supersedes: ["0002"]
superseded_by: null
related: ["0002", "0003", "0014", "0017", "0018", "0020", "0021", "0023"]
tags: [protocol, streaming, events, ai-sdk, channels, l3, l4]
---

# 0022 · Stream protocol v1 — AI SDK UIMessage base with data-z:* Zund extensions

Date: 2026-04-17
Status: accepted
Supersedes: ADR 0002 (canonical stream protocol)
Related: ADR 0003 (runtime interface), ADR 0014 (message endpoint), ADR 0017
(humans as fleet members), ADR 0018 (agent lifecycle), ADR 0020 (plugin
architecture), ADR 0021 (console consolidation), ADR 0023 (task queue +
dispatcher — consumes the `data-z:task:*` events defined here and is the
authoritative source for task state semantics and transitions)

## Context

ADR 0002 set the direction: define `zund://stream/v1` as a **Zund-owned
canonical event vocabulary** with per-runtime translators. The translator
shipped as an identity passthrough; the full event catalog was deferred
pending multi-runtime learnings. Three things have changed since:

1. **AI SDK's `UIMessage` stream is now the de facto standard.** Multiple
   consumer libraries speak it natively (AI Elements, assistant-ui, Vercel's
   `useChat`). It has the exact shape ADR 0002 envisioned (start/delta/end
   lifecycle, tool input/output split, `data-*` extension channel). A
   Zund-owned parallel vocabulary with ~95% identical shape would incur
   translator cost and ecosystem isolation for cosmetic naming differences.

2. **Console is committed to AI Elements.** AI Elements is shadcn-style
   (components copied into the repo, owned locally) and ships domain-aligned
   primitives — `Artifact`, `Agent`, `Terminal`, `File Tree`, `Chain of
   Thought`, `Tool`, `Sandbox`. These map directly onto Zund concepts. AI
   Elements consumes AI SDK's `UIMessage` stream via `useChat`.

3. **Consumer surface analysis.** Near-term wire consumers are:
   (a) CLI, (b) console, (c) internal channel adapters (Slack, WhatsApp,
   Telegram) we will write ourselves. All first-party. No external SDK
   consumers planned in the current window. This collapses the main
   motivation for a Zund-owned vocabulary (protect third-party consumers
   from ecosystem churn).

Additionally, **Hermes has a mature, validated channel gateway** covering
17+ platforms (Slack, Telegram, WhatsApp, Discord, Matrix, Signal, SMS,
Email, Feishu, DingTalk, WeCom, etc.). Its delivery pattern (progressive
message edit, rate-limited, tool-boundary segmentation) is directly
applicable to Zund's channel adapters and informs this decision.

## Decision

### Summary

Adopt **AI SDK's `UIMessage` stream** as the base wire format for
`zund://stream/v1`. Add Zund-native concepts (artifacts, memory, tasks,
agent lifecycle, fleet ops) as `data-z:<domain>:<event>` parts — AI SDK's
`data-*` extension channel, namespaced under `z:` to keep the Zund surface
cleanly partitioned. No parallel Zund-owned vocabulary; no mandatory
translator layer.

### 1. Protocol name and headers

- Protocol name stays **`zund://stream/v1`**. What Zund owns is the
  *composition* — AI SDK UIMessage base + the `data-z:*` catalog +
  semantics around lifecycle, task state, and fleet operations. The base
  format is borrowed; the contract is Zund's.
- Version negotiation: `x-zund-stream: v1` response header on every SSE.
- Transport: SSE over HTTP (per ADR 0014).

### 2. Base vocabulary: AI SDK UIMessage stream

Runtimes and the daemon emit `UIMessagePart` events directly. The core
parts we use (see AI SDK docs for full shape):

```
# Message & step lifecycle
start              { messageId }
start-step         { messageId? }
finish-step        { finishReason, usage, isContinued }
finish             { finishReason, usage }

# Text streaming
text-start         { id }
text-delta         { id, textDelta }
text-end           { id }

# Reasoning / thinking
reasoning-start    { id }
reasoning-delta    { id, delta }
reasoning-end      { id }

# Tool lifecycle
tool-call-start    { id, toolCallId, toolName }
tool-call-delta    { toolCallId, argsText }
tool-call-end      { toolCallId }
tool-result        { toolCallId, result, isError? }

# Sources & files
source             { source: { sourceType, id, url, title? } }
file               { file: { mimeType, data } }

# Terminal
error              { errorText }
```

Clients (console via AI Elements, CLI, channel adapters) consume these as
`UIMessagePart`. No renaming, no shim layer.

### 3. Zund-native extensions: `data-z:<domain>:<event>`

Zund-native concepts go on the **`data-*` extension channel**, namespaced
with a leading `z:` segment. Convention: `data-z:<domain>:<event>`, where
`<domain>` is a noun (`agent`, `artifact`, `memory`, `task`, `fleet`) and
`<event>` is a dash-cased verb phrase (`started`, `apply-completed`).

**Why colons as namespace separators:** unambiguous visual split between
namespace marker, domain, and event name. Grep-friendly
(`data-z:task:` gets every task event). Valid after the `data-` prefix
(AI SDK treats everything after `data-` as an opaque type key).

**Full v1 catalog:**

```
# Agent lifecycle (also fires on fleet broadcast per §6)
data-z:agent:started    { agent, runtime, at }
data-z:agent:stopped    { agent, reason: "idle" | "error" | "terminated", at }
data-z:agent:error      { agent, code, message, recoverable, at }

# Artifact lifecycle — two-state; failure surfaces via wrapping `error` event
data-z:artifact:creating { id, kind, mimeType, label, agent }
data-z:artifact:created  { id, url, kind, mimeType, label, agent }

# Memory — coarse; client refetches via API to see what changed
data-z:memory:updated    { agent, scope, kind }

# Task state machine — see §4. ADR 0023 is authoritative on state semantics.
data-z:task:scheduled    { taskId, scheduledAt, source, prompt }
data-z:task:queued       { taskId, source, prompt }
data-z:task:dispatching  { taskId }
data-z:task:pending      { taskId, pendingReason, suggestion }
data-z:task:dispatched   { taskId, agent, dispatchReasoning }
data-z:task:running      { taskId, agent }
data-z:task:blocked      { taskId, agent, blockedReason }
data-z:task:completed    { taskId, result }
data-z:task:failed       { taskId, code, message }
data-z:task:cancelled    { taskId, cancelledBy }

# Fleet operations
data-z:fleet:apply-started   { fleet, agentsCreated, agentsUpdated, agentsDeleted }
data-z:fleet:apply-completed { fleet, result }
data-z:fleet:health          { agents: [{ name, status }] }
```

Every `data-z:*` event carries sufficient identity fields (agent, taskId,
id) for client-side filtering. Timestamps on events are daemon-generated.

### 4. Task state machine (wire projection)

The task queue owns 10 states across four phases. **ADR 0023 is
authoritative on state meanings, transitions, and storage.** The table
below is the wire projection: exactly one `data-z:task:*` event fires on
entering each state.

| State | Phase | Wire event |
|---|---|---|
| `scheduled`   | pre-dispatch | `data-z:task:scheduled`   |
| `queued`      | pre-dispatch | `data-z:task:queued`      |
| `dispatching` | dispatch     | `data-z:task:dispatching` |
| `pending`     | dispatch     | `data-z:task:pending`     |
| `dispatched`  | execution    | `data-z:task:dispatched`  |
| `running`     | execution    | `data-z:task:running`     |
| `blocked`     | execution    | `data-z:task:blocked`     |
| `completed`   | terminal     | `data-z:task:completed`   |
| `failed`      | terminal     | `data-z:task:failed`      |
| `cancelled`   | terminal     | `data-z:task:cancelled`   |

Two states are easy to confuse: **`pending` is pre-assignment** (no
agent match yet, awaits fleet change); **`blocked` is post-assignment**
(agent running but paused on an external signal, resumable). Distinct
events so UI can render them differently — `pending` as a queue badge
with "needs capability X", `blocked` as an inline status on the running
message with "awaiting approval."

See ADR 0023 §Task lifecycle for the full transition diagram, state
entry/exit conditions, and queue storage schema.

### 5. Persistence and resume semantics

- **No mid-stream resume.** The server completes every response
  regardless of client connection state. When a client drops, the
  response keeps running server-side.
- **Reload path:** on reconnect, the client fetches the completed
  message via the persistent-session API (JSONL-backed for Pi, per
  ADR 0009). The console reload shows the last message as it is in
  storage.
- **Consequence:** no event IDs on the wire. No `Last-Event-Id` replay.
  No resume-specific envelope fields. SSE is "live tail", not "reliable
  delivery".

This is a deliberate simplification enabled by the first-party consumer
surface. If external consumers appear later with unreliable networks
(mobile SDKs, flaky integrations), reintroduce event IDs as a v1-additive
extension.

### 6. Streams and endpoints

Three SSE endpoints, with intentional event overlap:

- **`GET /v1/agents/:name/stream`** — per-agent SSE. Carries:
  - Message/tool/text events for the agent's current response
  - `data-z:artifact:*` tied to that agent's work
  - `data-z:memory:updated` for that agent
  - **That agent's** `data-z:agent:started`/`stopped`/`error` events
    (so a client watching one agent sees a complete timeline).

- **`GET /v1/events`** — fleet broadcast SSE. Carries summary/lifecycle
  across all agents:
  - `data-z:agent:*` for every agent
  - `data-z:task:*` state transitions (all tasks)
  - `data-z:fleet:*` operations
  - Each event includes `agent` / `taskId` identity fields; client
    filters.

- **`GET /v1/tasks/:id/stream`** — dedicated per-task SSE for focused
  views (e.g., console drilling into one long-running task). Carries the
  full chatter for that task: interim text/tool events + terminal state.

**Duplication is deliberate.** The same `data-z:agent:error` fires on
both the per-agent stream and the fleet broadcast. Clients pick the
stream that matches their view; they do not need to multiplex two
streams for a complete picture. Wire cost is negligible; UX clarity is
high.

### 7. Non-streaming responses

`POST /v1/agents/:name/message?stream=false` returns a final JSON object
(completed message + tool calls + artifact summaries) — not a serialized
event stream. The UIMessage vocabulary applies to SSE only.

Tasks that run asynchronously (completion minutes or hours later) return
a `{ taskId }` immediately; their progression surfaces on the fleet
broadcast (`data-z:task:*`) and on `/v1/tasks/:id/stream` for the
focused view. The task-completed "notification-bar" UX in the console
reads from fleet broadcast.

### 8. Translator — collapse with optional transform hook

- **Default path:** runtimes emit `UIMessagePart` + `data-z:*` parts
  directly. The daemon's stream path is identity passthrough.
- **Escape valve:** a runtime plugin may register an optional
  `transform(nativeEvent) → UIMessagePart[]` function if its native
  shape differs from UIMessage. The transform is wired into the runtime
  plugin contract (per ADR 0020), not a daemon-internal registry.
- **Pi runtime (current):** emits Anthropic-shaped events that map 1:1 to
  UIMessage. No transform required.
- **Third-party runtimes** (Hermes, OpenClaw, etc.) ship their own
  transform if their native event shape differs. This keeps the daemon
  free of per-runtime mapping tables.

This is a softer version of ADR 0002's translator seam: the seam exists
(pluggable via runtime plugin contract), but it is off by default instead
of a mandatory layer.

### 9. UI library: AI Elements

The console uses **AI Elements** (shadcn distribution — components
copied into `packages/console/src/components/ai-elements/`). Rationale:

- **Domain fit:** AI Elements ships `Artifact`, `Agent`, `Terminal`,
  `File Tree`, `Chain of Thought`, `Tool`, `Sandbox` primitives. These
  are Zund's concepts already.
- **Ownership:** shadcn-style = we own the component code. No upstream
  API break risk; we customize aggressively for artifact previews,
  memory diffs, task cards, fleet views.
- **Protocol alignment:** AI Elements consumes AI SDK's `UIMessage`
  stream via `useChat` — zero adaptation from our wire.
- **`data-z:*` rendering:** custom React components in the console's
  data-part dispatcher render each `data-z:*` variant (artifact card,
  memory-updated badge, task state pill, agent-lifecycle toast).

Alternatives rejected:

- **assistant-ui:** better thread/multi-step state machine out of the
  box, but Zund's console is agent-ops-focused (artifacts and tasks
  first, chat second) and doesn't need thread branching, message
  editing, or regeneration. Library coupling > shadcn ownership for
  our case.
- **Build our own:** no, AI Elements does it for us and we can
  modify anything we don't like.

### 10. Channel adapters (reference: Hermes gateway pattern)

Channel adapters (Slack, WhatsApp, Telegram, Discord, Matrix, SMS,
Email — planned for a later milestone) are **clients of our SSE
streams**, not sources. They consume `UIMessage` + `data-z:*` and
translate down to platform-specific primitives.

**Hermes has already solved this** across 17+ platforms. Their pattern
(see `~/dev/lw/hermes-agent/gateway/`):

- **`gateway/platforms/`** — one file per platform implementing a
  common `base.py` interface. Includes `slack.py`, `telegram.py`,
  `whatsapp.py`, `discord.py`, `matrix.py`, `signal.py`, `mattermost.py`,
  `email.py`, `sms.py`, `feishu.py`, `dingtalk.py`, `wecom.py`,
  `weixin.py`, `qqbot.py`, `bluebubbles.py`, `homeassistant.py`,
  `webhook.py`. Covers every mainstream channel.
- **`gateway/stream_consumer.py`** — the delta-to-chat bridge:
  - Send initial message → `editMessageText` repeatedly with streamed
    tokens.
  - Rate-limited (default 1s edit interval, 40-char buffer threshold).
  - Tool boundaries finalize the current message and start a new one
    (text appears below tool progress, not mixed in).
  - Sync-to-async bridge: worker thread fires deltas →
    `queue.Queue` → async platform task.
- **`gateway/delivery.py`** — target routing with
  `"platform:chat_id[:thread_id]"` syntax, `"origin"` (reply to source),
  `"local"` (file log), and per-platform "home channels."
- **`gateway/pairing.py`**, **`session.py`**, **`mirror.py`** — session
  state, user pairing (link Slack user ↔ agent), cross-platform mirroring.

**Zund's channel adapter reference shape:**

```
zund SSE (UIMessage + data-z:*)
          │
          ▼
┌──────────────────────────────┐
│ StreamConsumer (rate-limited)│
│  - buffers text-delta        │
│  - flushes on:               │
│      - 1s interval           │
│      - tool-call-start       │
│      - text-end              │
│      - finish                │
└──────────────────────────────┘
          │
          ▼ (platform-specific send/edit)
     Slack · Telegram · WhatsApp · …
```

OpenClaw's channel story was not audited in this ADR — defer to when we
actually ship a channel adapter. Hermes' pattern is sufficient as a v1
reference.

## Consequences

**Makes easier:**

- Console builds on a maintained, open standard (AI SDK UIMessage) with
  existing React component ecosystems.
- No translator layer to design/maintain in the default case — Pi
  emits directly.
- Zund-native concepts are cleanly namespaced and extensible (`data-z:*`).
- Channel adapters follow a known-working pattern (Hermes gateway).
- Reload semantics collapse to "fetch from JSONL" — no wire-side replay
  machinery.
- AI Elements' domain-aligned primitives accelerate console build-out.

**Makes harder:**

- Zund's wire tracks AI SDK's UIMessage evolution. Breaking changes in
  the AI SDK protocol propagate into Zund. **Mitigations:** AI SDK's
  UIMessage is a published protocol with multiple implementations; it's
  stabilizing; backward-compatibility is a published concern. Worst
  case, we pin to a snapshot and write a transform.
- Runtime authors must speak UIMessage (or ship a transform). Added
  burden on third-party runtime plugins (ADR 0020). **Mitigation:** the
  transform hook exists.
- The `data-z:*` catalog is Zund's responsibility. Versioning discipline
  from ADR 0002 still applies to the extension layer.
- Colon-in-type-name (`data-z:task:completed`) is unusual in the JS
  ecosystem. Tooling that splits identifiers on non-alphanumerics may
  need a minor adjustment. **Mitigation:** AI SDK's `onData` handler
  uses the whole suffix as an opaque key — no parsing involved.

## Implementation notes

1. **`packages/core/src/contracts/events.ts`:** replace `CanonicalEvent`
   with a re-export of AI SDK's `UIMessagePart` + a `ZundDataPart`
   discriminated union for every `data-z:*` variant in §3.
2. **`packages/daemon/src/stream/translator.ts`:** default translator is
   identity. Add an optional `RuntimeTransform` interface; wire it into
   the runtime plugin contract (ADR 0020).
3. **New endpoint: `GET /v1/tasks/:id/stream`** — dedicated per-task SSE.
4. **Console:** install `ai`, `@ai-sdk/react`, AI Elements registry.
   Add `data-z:*` renderers:
   - `ArtifactCreatedCard`
   - `MemoryUpdatedBadge`
   - `TaskStatePill`
   - `AgentLifecycleToast`
   - `FleetApplyTimeline`
5. **Mark ADR 0002 as superseded** (done in this ADR's landing commit).
   Frontmatter set to `status: superseded`, `superseded_by: "0022"`.
6. **Archive** `docs/reference/stream-events-thought-process.md` to
   `docs/archive/stream-events-thought-process.md` — it is the reasoning
   trail that fed this decision.

## Open questions (narrow)

- **Event ID policy for internal telemetry/logging.** The wire has no
  event IDs, but the daemon may add internal correlation IDs for
  tracing. Not user-visible; decide when observability work starts.
- **Backpressure / cancellation.** SSE close is the only cancellation
  signal. No explicit abort event on the wire. Revisit only if real
  slow-consumer problems emerge.
- **Breaking-change policy.** Clients MUST ignore unknown event types
  and unknown fields. v2 is triggered only by semantic changes to
  existing events or removal — additions are v1-compatible.
- **OpenClaw audit.** Its channel integration story was not reviewed.
  Audit when we actually ship a non-Hermes channel adapter.

## References

- **AI SDK UIMessage stream:** https://ai-sdk.dev/
- **AI Elements:** https://elements.ai-sdk.dev/
- **Hermes gateway (channel adapter reference):**
  `~/dev/lw/hermes-agent/gateway/` — in particular:
  - `stream_consumer.py` — progressive-edit, rate-limited delta
    streaming across platforms
  - `platforms/` — 17+ platform implementations sharing a common base
  - `delivery.py` — target routing semantics
  - `session.py`, `pairing.py`, `mirror.py` — session and identity
    bridging
- **Thought-process doc (reasoning trail):**
  `docs/archive/stream-events-thought-process.md`
- **Superseded:** ADR 0002 — Canonical stream protocol (zund://stream/v1)
