---
id: "0023"
title: "Task queue + LLM dispatcher with hint-based overrides"
date: 2026-04-17
status: draft
implementation: not-started
supersedes: []
superseded_by: null
related: ["0001", "0003", "0004", "0017", "0018", "0020", "0022"]
tags: [queue, dispatcher, tasks, l3, orchestration, capability-index]
---

# 0023 · Task queue + LLM dispatcher with hint-based overrides

Date: 2026-04-17
Status: draft
Related: ADR 0001 (four-layer architecture), ADR 0003 (runtime interface), ADR 0004 (Incus ephemeral), ADR 0017 (humans as fleet members), ADR 0018 (agent lifecycle), ADR 0020 (plugin architecture), ADR 0022 (stream protocol — defines `data-z:task:*` wire events that emit on every state transition defined here)

## Context

Zund's fleet today is **declarative-only**: you define agents in YAML,
`zund apply` reconciles containers, agents run forever. There is no
imperative path for "a task appeared — who should handle it?"

ADR 0001 places the dispatcher, triggers, and task queue in L3
(Orchestration). `roadmap/next.md` specifies the L3 build-out as the
next major feature: task queue, capability index, dispatcher, pending
reprocessor, triggers, result callback. None of this exists in code.

Without it, three patterns that teams need are impossible:

1. **Ad-hoc task submission.** "Review this report" or "Fix the build"
   has nowhere to land. The user must know which agent to message
   directly (ADR 0014).
2. **Event-driven work.** Cron, webhooks, and agent-chain outputs have
   no structured way to create work items that flow through the fleet.
3. **Self-healing when capability is missing.** If no agent can do a
   task, the task disappears — it's not queued, there's no reason
   recorded, no reprocessing when new agents appear.

## Decision

Introduce three L3 primitives that work together:

1. **Task Queue** — L2-backed durable store for tasks with free-text
   prompts, optional hint overrides, and explicit status lifecycle.
2. **Capability Index** — a fleet-wide context document rebuilt on
   `zund apply`, consumed by the dispatcher (and, eventually, by
   agents discovering each other).
3. **LLM Dispatcher** — resolves a task to an agent by making an LLM
   call over the task prompt + capability index. Hints in the task
   can short-circuit the LLM call entirely.

### Task submission — free text + optional hints

Tasks arrive as free-text prompts. The user does not declare
structured capabilities — the dispatcher **derives** what's needed from
the prompt and matches it against the fleet.

A `hints` map provides **override keys** that short-circuit or
influence dispatching without going through the LLM. The only
initially defined hint is `agent`, but the map is open-ended for
future use.

```typescript
// POST /v1/tasks
interface TaskIngress {
  /** What needs to be done — free text, the only required field. */
  prompt: string;

  /** Additional context (attached files, URLs, references). */
  context?: string;

  /** Where this task came from. */
  source: "api" | "cron" | "webhook" | "agent-chain";

  /**
   * Key-value overrides that bypass or influence dispatcher logic.
   *
   * Recognized keys:
   *   agent: string   — skip LLM routing, assign directly to this agent
   *
   * Unknown keys are stored but ignored by the default dispatcher.
   * Future keys might include: priority, model, lifecycle, timeout,
   * required_secrets, team, etc.
   */
  hints?: Record<string, string>;
}
```

**Hint semantics are strict: presence means override, absence means
dispatch normally.** If `hints.agent` is set, the dispatcher does not
run — the task is assigned to that agent immediately. This lets callers
who already know the target agent (cron, agent chains, UI selection)
skip the LLM call, saving latency and cost.

### Task lifecycle

The task state machine has **10 states** across four phases: pre-dispatch,
dispatch, execution, and terminal. Every state transition emits a
`data-z:task:*` wire event per ADR 0022.

```
PRE-DISPATCH           DISPATCH               EXECUTION             TERMINAL
─────────────          ─────────────          ─────────────         ─────────────

 ┌───────────┐
 │ scheduled │ ── (at scheduledAt) ──┐
 └─────┬─────┘                        │
       │                              ▼
       │                       ┌──────────┐
       └──→  (cancel)          │  queued   │ ← ingress (API/cron/webhook/chain)
                               └─────┬────┘
                                     │
                                     ▼
                            ┌────────────────┐
                            │  dispatching   │ ← dispatcher LLM call
                            └──┬──────┬──────┘
                               │      │
                       match found   no match
                               │      │
                               ▼      ▼
                       ┌────────────┐ ┌──────────┐
                       │ dispatched │ │  pending  │ ← awaits capability change
                       └─────┬──────┘ └─────┬────┘
                   (container │              │  ▲
                    warming)  │              │  │  (reprocessed on fleet
                              ▼              └──┘   apply / new skill)
                         ┌────────┐
                         │ running │ ◄──────┐
                         └───┬────┘          │
                             │                │ (resumed)
                             ▼                │
                         ┌─────────┐          │
                         │ blocked  │─────────┘  ← awaits external input
                         └────┬─────┘              (human approval,
                              │                    rate limit, timeout)
                              ▼
                   ┌──────────┬─────────┐
                   ▼          ▼         ▼
            ┌───────────┐ ┌────────┐ ┌───────────┐
            │ completed │ │ failed │ │ cancelled │
            └───────────┘ └────────┘ └───────────┘
```

Two states carry a reason field:

- `pending.pendingReason` + `pending.suggestion` — why no agent matched
  and what could resolve it. Reprocessor re-evaluates on fleet apply.
- `blocked.blockedReason` — what the running agent is waiting for
  (e.g. `"awaiting human approval"`, `"rate limited"`,
  `"tool timeout"`). Resumable — the agent can leave `blocked` and
  return to `running` multiple times.

`pending` and `blocked` are deliberately distinct: `pending` is
**pre-assignment** (no home found yet); `blocked` is
**post-assignment, mid-execution** (running, paused on something
external). Same task can visit both — first `pending` (until a
capability appears), then `running`, then `blocked` (awaiting input),
then `running` again, then `completed`.

### Statuses

| Status | Phase | Meaning | Next states |
|--------|-------|---------|-------------|
| `scheduled` | pre-dispatch | Created, will become `queued` at `scheduledAt`. | `queued`, `cancelled` |
| `queued` | pre-dispatch | Ready for dispatch; awaiting dispatcher pickup. | `dispatching`, `cancelled` |
| `dispatching` | dispatch | Dispatcher LLM call in progress. Transient. | `dispatched`, `pending`, `failed` |
| `pending` | dispatch | No agent matched; awaits capability change. Carries `pendingReason` + `suggestion`. | `dispatching` (reprocessed), `cancelled` |
| `dispatched` | execution | Assigned to agent; container warming / session attaching. | `running`, `failed`, `cancelled` |
| `running` | execution | Agent actively processing. | `completed`, `failed`, `cancelled`, `blocked` |
| `blocked` | execution | Running but awaiting external input (approval, rate limit, tool timeout). Carries `blockedReason`. Resumable. | `running`, `failed` (timeout), `cancelled` |
| `completed` | terminal | Agent finished successfully. | — |
| `failed` | terminal | Agent errored, container failed to start, or blocked-too-long timeout. | — (or manual retry) |
| `cancelled` | terminal | User/system terminated before completion. | — |

`dispatching` and `dispatched` are distinct: `dispatching` is the
server-side micro-state during the dispatcher LLM call (visible on the
wire so consoles can render "routing…"); `dispatched` is post-assignment
while the agent's container cold-starts (Pi warm-up is non-instant —
ADR 0007).

### Queue schema (SQLite)

```sql
CREATE TABLE tasks (
  id TEXT PRIMARY KEY,                -- UUID
  source TEXT NOT NULL,               -- 'api' | 'cron' | 'webhook' | 'agent-chain'
  prompt TEXT NOT NULL,               -- free-text task description
  context TEXT,                       -- JSON: attached files, URLs, references
  hints TEXT,                         -- JSON: key-value overrides (e.g. {"agent":"researcher"})
  status TEXT NOT NULL DEFAULT 'queued',
                                      -- one of: scheduled | queued | dispatching |
                                      -- pending | dispatched | running | blocked |
                                      -- completed | failed | cancelled
  scheduled_at TEXT,                  -- when status='scheduled', transition time
  assigned_agent TEXT,                -- agent name once dispatched
  dispatch_reasoning TEXT,            -- LLM's explanation for the assignment
  pending_reason TEXT,                -- why no agent matched (status='pending')
  suggestion TEXT,                    -- how to resolve the pending state
  blocked_reason TEXT,                -- why agent is blocked (status='blocked')
  cancelled_by TEXT,                  -- user/system that cancelled
  result TEXT,                        -- JSON: task output, artifact references
  error TEXT,                         -- error message on failure
  created_at TEXT NOT NULL,
  dispatched_at TEXT,
  started_at TEXT,
  completed_at TEXT,
  priority INTEGER DEFAULT 0
);

CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_agent_active ON tasks(assigned_agent)
  WHERE status IN ('running', 'blocked');
CREATE INDEX idx_tasks_scheduled ON tasks(scheduled_at)
  WHERE status = 'scheduled';
CREATE INDEX idx_tasks_pending ON tasks(status)
  WHERE status = 'pending';
```

### TaskQueue contract

The queue is a L2 state concern, abstracted behind an interface in
`@zund/core/contracts/queue.ts`. Default implementation is SQLite.
The contract follows the same pluggable pattern as MemoryStore,
ArtifactStore, etc. (ADR 0015, ADR 0020).

```typescript
// packages/core/src/contracts/queue.ts

export type TaskStatus =
  | "scheduled"   // future trigger, becomes 'queued' at scheduledAt
  | "queued"      // ready for dispatcher pickup
  | "dispatching" // dispatcher LLM call in progress
  | "pending"     // no capability match; awaits fleet change
  | "dispatched"  // assigned to agent; container warming
  | "running"     // agent actively processing
  | "blocked"     // running but awaiting external input (resumable)
  | "completed"   // terminal success
  | "failed"      // terminal error
  | "cancelled";  // terminal, user/system terminated

export interface Task {
  id: string;
  source: TaskSource;
  prompt: string;
  context?: string;
  hints?: Record<string, string>;
  status: TaskStatus;
  scheduledAt?: string;       // set when status='scheduled'
  assignedAgent?: string;
  dispatchReasoning?: string;
  pendingReason?: string;     // set when status='pending'
  suggestion?: string;        // set when status='pending'
  blockedReason?: string;     // set when status='blocked'
  cancelledBy?: string;       // set when status='cancelled'
  result?: string;
  error?: string;
  createdAt: string;
  dispatchedAt?: string;
  startedAt?: string;
  completedAt?: string;
  priority: number;
}

export type TaskSource = "api" | "cron" | "webhook" | "agent-chain";

export interface TaskIngress {
  prompt: string;
  context?: string;
  source: TaskSource;
  hints?: Record<string, string>;
  priority?: number;
  scheduledAt?: string;       // if present, task starts in 'scheduled' state
}

export interface TaskQueue {
  /** Enqueue a new task. Returns the task ID. */
  enqueue(task: TaskIngress): Promise<string>;

  /** Claim the next batch of queued tasks for dispatching. */
  dequeue(limit?: number): Promise<Task[]>;

  /** Update a task (status transitions, assignment, result, etc.). */
  update(id: string, patch: Partial<Task>): Promise<void>;

  /** Get a task by ID. */
  get(id: string): Promise<Task | null>;

  /** List tasks, optionally filtered by status. */
  list(filter?: { status?: TaskStatus; agent?: string }): Promise<Task[]>;

  /** Get all pending tasks (for reprocessing). */
  pending(): Promise<Task[]>;

  /** Re-dispatch pending tasks that may now be matchable. Returns count re-queued. */
  reprocessPending(): Promise<number>;
}
```

**Why SQLite, not Redis/BullMQ:** Zund is single-host (ADR 0004).
SQLite WAL mode handles thousands of concurrent reads and hundreds of
writes per second — more than sufficient. Adding Redis as a dependency
for the queue would break the "zero infra beyond Incus" constraint.
If/when multi-node fleets ship as a commercial feature, a
`RedisTaskQueue` or `PostgresTaskQueue` implements the same contract.
The abstraction is the point; SQLite is the sensible default.

### Capability index

A derived document (not a database table) rebuilt on every `zund apply`.
It serializes the fleet's agent definitions, skills, tools, and
descriptions into a compact text block that the dispatcher LLM consumes
as context.

```yaml
# Capability index — auto-generated, never hand-edited
# Rebuilt on: zund apply
# Consumed by: L3 dispatcher

agents:
  - name: researcher
    model: claude-sonnet-4
    lifecycle: persistent
    skills: [search-web, summarize, pdf-read]
    tools: [bash, read, write]
    description: "Deep research and synthesis agent"

  - name: coder
    model: claude-sonnet-4
    lifecycle: persistent
    skills: [code-review, test-write, debug]
    tools: [bash, read, edit, write]
    description: "Code generation, review, and debugging"

  - name: writer
    model: claude-sonnet-4
    lifecycle: persistent
    skills: [summarize, draft-report, copy-edit]
    tools: [read, write]
    description: "Long-form content and documentation"

humans:
  - name: nachiket
    capabilities: [code-review, deploy-approval, business-decisions]
    channels: [telegram, email]
    availability: "Asia/Kolkata 09:00-19:00"
```

The index lives in the daemon's in-memory state (rebuilt on apply, not
persisted to disk — it's a derived view). The dispatcher reads it from
memory when constructing the LLM prompt.

Future extensions: skills registry entries with descriptions, secret
availability per agent (dispatcher can factor in "this agent has AWS
credentials"), MCP server capabilities. The index grows richer as
the fleet model does.

### Dispatcher protocol

The dispatcher is a small LLM call that takes the task prompt + context
and the capability index, and returns a routing decision.

```typescript
// packages/core/src/contracts/dispatcher.ts

export interface DispatchRequest {
  task: Task;
  capabilityIndex: string;   // serialized index doc
}

export interface DispatchResult {
  /** Agent name to assign, or null if no match. */
  agent: string | null;
  /** LLM's reasoning for the assignment (stored on the task). */
  reasoning: string;
  /** If agent is null, why no agent could handle it. */
  pendingReason?: string;
  /** If agent is null, what could be done to resolve it. */
  suggestion?: string;
}

export interface Dispatcher {
  /** Resolve a task to an agent (or explain why not). */
  dispatch(req: DispatchRequest): Promise<DispatchResult>;
}
```

**Hint override flow:**

```
Task arrives
    │
    ├── hints.agent present?
    │       │
    │     yes → assign directly, skip LLM call
    │       │
    │       ▼
    │     Task → status: running, assigned_agent: hints.agent
    │
    └── no hints.agent
            │
            ▼
        Dispatcher LLM call
        input: task.prompt + task.context + capability_index
            │
        ┌───┴──────────┐
        ▼               ▼
    agent matched    no match
        │               │
        ▼               ▼
    status: running  status: pending
    assigned_agent   pendingReason + suggestion
```

**LLM model for the dispatcher:** the dispatcher needs to be fast and
cheap, not clever. Default: a small model (e.g. `gpt-4o-mini`,
`claude-haiku`) configured in fleet YAML. The dispatcher is a service
plugin (ADR 0020) — `dispatcher-llm` is the default, `dispatcher-rules`
(a deterministic rule engine) is a potential future alternative.

```yaml
# fleet/plugins.yaml (dispatcher config)
config:
  dispatcher-llm:
    model: "claude-haiku"
    # or model: "ollama://localhost:11434/llama3"
    temperature: 0.1
```

### Pending reprocessor

When a task lands in `pending`, it is not abandoned. Two events trigger
re-evaluation:

1. **`zund apply`** — the capability index is rebuilt. Any pending
   tasks whose `pendingReason` may now be resolvable are re-queued
   (`pending` → `queued`).
2. **Explicit `POST /v1/tasks/reprocess`** — manual trigger (useful
   during development or after installing a new skill).

The reprocessor does not batch-retry everything — it walks pending
tasks, and for each, re-runs the dispatcher with the updated capability
index. Tasks that still can't be dispatched stay pending with updated
reasoning.

### Ephemeral agent spawning (Phase 2)

When the dispatcher finds no matching persistent agent, the far-future
path is to compose and spawn a Spot-tier ephemeral agent (ADR 0018):

```
No match found
    │
    ▼
Dispatcher: "I need an agent with skills X, Y, Z"
    │
    ▼ (future)
Role auto-derivation → new Role YAML → apply → Spot agent spawned
    │
    ▼
Task assigned to ephemeral agent → completes → container reaped
```

This is deliberately Phase 2+. The architecture supports it because:
- `AgentResource.lifecycle: "ephemeral"` already exists in fleet types
- `cloneEphemeral()` exists in experiments (POC 06)
- Role YAML is already a fleet kind, so programmatic `apply` is feasible

Phase 1 strictly assigns to existing agents or marks as pending.

### Wire events

Every state transition emits a `data-z:task:<state>` event — one event
per state entry. **See ADR 0022 §3 for the full event catalog and
payload shapes, and ADR 0022 §4 for the state-to-event mapping table.**

Events surface on two streams (per ADR 0022 §6):

- **Fleet broadcast** (`GET /v1/events`) — every task transition,
  across all tasks. Consumed by channel bots, notification UIs, and the
  console's task queue view.
- **Per-task stream** (`GET /v1/tasks/:id/stream`) — a single task's full
  timeline, including the nested text/tool `UIMessage` events once the
  task is `running`.

Between events, clients refetch from `GET /v1/tasks/:id` to reconcile
state if they drop the stream.

### API surface

```
POST   /v1/tasks                  — submit a task (returns task ID)
GET    /v1/tasks                  — list tasks (filter: ?status=queued)
GET    /v1/tasks/:id              — get task details
POST   /v1/tasks/:id/retry       — re-queue a failed task
POST   /v1/tasks/reprocess        — re-dispatch all pending tasks
DELETE /v1/tasks/:id              — cancel/remove a task
GET    /v1/capability-index       — inspect the current index (debug)
```

Tasks feed from multiple sources, all producing the same `TaskIngress`:

| Source | Trigger | Notes |
|--------|---------|-------|
| API | `POST /v1/tasks` | User or external system |
| Cron | `TriggerConfig.cron` on agent YAML | Scheduler enqueues on schedule |
| Webhook | `POST /v1/triggers/:name` | GitHub, Stripe, etc. |
| Agent chain | Agent emits `zund:chain` event | Output of one task feeds the next |

### Result callback

Ephemeral agents need a way to report results. The daemon injects
`ZUND_CALLBACK_URL=http://host.zund.internal:4000/v1/tasks/:id/result`
as an env var at launch. The agent (or its runtime bridge) POSTs the
result when done. Persistent agents report through the existing
message stream (ADR 0014).

```typescript
// POST /v1/tasks/:id/result
interface TaskResult {
  output: string;                     // text result
  artifacts?: Array<{ sha256: string; label: string }>;
  status: "completed" | "failed";
  error?: string;                     // if failed
}
```

## Challenges and open questions

### Hint vocabulary growth

`hints` is an open map. `agent` is the first defined key. Future keys
might include: `priority`, `model`, `lifecycle`, `timeout`,
`required_secrets`, `team`. The risk is uncontrolled vocabulary —
every key becomes an implicit contract the dispatcher must honor.

**Mitigation:** the dispatcher ignores unknown keys (stored, not
acted on). New keys are only promoted to "recognized" with an ADR
amendment or a new ADR. The initial contract is: `agent` = skip
routing, assign directly. Everything else is stored passthrough.

### Dispatcher LLM cost and latency

Every dispatched task (without `hints.agent`) requires an LLM call.
At high throughput, this is both a latency and cost concern.

**Mitigations:**
- Small, cheap model (haiku-class) for dispatching.
- Cache: if the same prompt pattern resolves to the same agent
  repeatedly, a local cache can short-circuit the LLM call.
  (Future optimization, not v1.)
- `hints.agent` lets high-volume sources (cron, agent chains) skip
  the LLM call entirely.

### Capability index staleness

The index is rebuilt on `zund apply`, not on every agent state change.
If an agent gains a skill in Open mode (ADR 0018) between applies, the
dispatcher won't know about it.

**Acceptable for v1.** Open-mode drift is exceptional; the default
path is YAML-declared capabilities. Phase 2 (proposal stream from
ADR 0018) may trigger index rebuilds on proposal approval, but that's
a conscious addition, not a v1 requirement.

### Concurrent dispatchers

Multiple dispatchers can consume from the same queue safely because
`dequeue()` claims tasks atomically (SQLite `UPDATE ... WHERE status =
'queued' RETURNING *`). Two dispatchers won't claim the same task.

However, multiple dispatchers serving the *same* capability index is
redundant unless there's domain specialization (e.g., one dispatcher
for code tasks, one for research). That's a Phase 2 concern. V1 ships
one dispatcher loop.

### Task→agent assignment when agent is busy

If the dispatched-to agent is mid-conversation, what happens? Options:
- Queue the task behind the agent's current work.
- Reject and re-dispatch to another agent.
- Spawn an ephemeral Spot instance of the same role.

**V1:** queue behind (the agent processes tasks sequentially per its
existing session model). The dispatcher does not check agent workload
in v1 — capacity-aware routing is Phase 2.

### Human dispatching (ADR 0017)

When the dispatcher routes a task to a human fleet member, the task
enters `running` with `assigned_agent` set to the human's name. The
human runtime (ADR 0017) delivers the prompt via a gateway channel
(Telegram, email). The task completes when the human replies or
times out (escalation policy).

This works architecturally but depends on the human runtime and
gateway plugins being implemented. Out of scope for v1 dispatcher,
but the task schema and dispatch protocol don't need changes to
support it — it's just another agent name to the dispatcher.

## Phased implementation

### Phase 1 — Queue + direct assignment (hint-only dispatch)

**Goal:** tasks can be submitted and assigned to a specific agent via
`hints.agent`. No LLM dispatcher yet.

**Scope:**
- `TaskQueue` contract in `@zund/core/contracts/queue.ts`.
- `SqliteTaskQueue` implementation (table in `memory.db` or new
  `queue.db`).
- `POST /v1/tasks` — ingest with `hints.agent` required (or status
  stays `queued` until dispatcher exists).
- `GET /v1/tasks`, `GET /v1/tasks/:id` — read.
- Task assigned immediately when `hints.agent` is present →
  `queued` → `dispatched` → `running` → `completed`/`failed`.
- Tasks without `hints.agent` stay `queued`.
- Result callback endpoint: `POST /v1/tasks/:id/result`.
- Phase 1 uses a **subset of the state machine**:
  `queued` → `dispatched` → `running` → terminal. The other states
  (`scheduled`, `dispatching`, `pending`, `blocked`) light up in later
  phases.
- Cancellation: `DELETE /v1/tasks/:id` → `cancelled` from any
  non-terminal state.

**Shippable outcome:** a working task queue. Cron and webhooks can
submit work. Explicit routing works. Everything else is queued and
waiting.

### Phase 2 — Capability index + LLM dispatcher

**Goal:** free-text tasks are automatically routed to the best agent.

**Scope:**
- Capability index builder (reads fleet YAML, emits serialized doc).
- `Dispatcher` contract in `@zund/core/contracts/dispatcher.ts`.
- `dispatcher-llm` implementation (small model, Ollama-first for
  local, Anthropic/OpenAI as fallback).
- Dispatch loop: poll queue → set `dispatching` → call dispatcher →
  transition to `dispatched` (match) or `pending` (no match).
- `dispatching` status on the wire (console renders "routing…").
- `pending` status + `pendingReason` + `suggestion` on the task.
- Pending reprocessor triggered on `zund apply` (→ back to
  `dispatching`).

**Shippable outcome:** the full dispatching loop. Users submit
free-text tasks and the fleet routes them. States added:
`dispatching`, `pending`.

### Phase 3 — Triggers feeding the queue

**Goal:** cron, webhooks, and agent chains create tasks automatically.
Deferred/scheduled tasks land in `scheduled` state.

**Scope:**
- Cron scheduler (in-process, reads `TriggerConfig.cron` from fleet
  YAML).
- Webhook endpoint: `POST /v1/triggers/:name` → enqueue task.
- Agent chain events: `zund:chain` event from agent result → enqueue
  next task.
- `scheduled` status: `TaskIngress.scheduledAt` puts the task in
  `scheduled` state until the scheduler promotes it to `queued`.
- All sources produce the same `TaskIngress`.

**Shippable outcome:** event-driven fleets. Cron jobs and webhooks
drive agent work without human initiation. State added: `scheduled`.

### Phase 4 — Capacity-aware routing + ephemeral spawn + blocked state

**Goal:** dispatcher considers agent workload, spawns ephemeral agents
for unmatched tasks, and the runtime supports pause/resume for
externally blocked work.

**Scope:**
- `Runtime.canHandle(task)` negotiation — ask agent if it can take
  the task given its current state (rate limits, workload, secret
  availability).
- Workload tracking: agents report active task count; dispatcher
  factors this into routing.
- Ephemeral agent spawning: when no persistent agent matches, compose
  a minimal Role YAML, apply, assign Spot instance.
- `blocked` status + `blockedReason`: runtime emits "I'm waiting on X"
  (human approval, rate limit, tool timeout). Task transitions
  `running` → `blocked` → `running` (resumed) until terminal state.
  Blocked-too-long timeout → `failed`.

**Shippable outcome:** the full self-healing, self-scaling queue.
New capabilities appear, pending tasks re-dispatch, overflow spawns
ephemeral agents, and long-running tasks can pause awaiting external
signals without losing their place. State added: `blocked`.

## Consequences

**Makes easier:**

- Imperative task flow on top of the existing declarative fleet.
  The fleet is still defined in YAML; the queue is how work **flows**
  through it.
- Free-text submission means zero learning curve for the caller —
  describe what you need, the fleet figures out who does it.
- `hints.agent` gives deterministic routing when the caller already
  knows the target (cron, UI selection, agent chains) — no wasted
  LLM calls.
- `pending` + `pendingReason` makes the queue self-documenting —
  you can always answer "why isn't this done?"
- Pending reprocessor makes the queue self-healing — new agents
  unblock stuck tasks.
- SQLite default keeps the infra story simple. Redis/Postgres queues
  slot in when scale demands it, with no API changes.

**Makes harder:**

- Dispatcher LLM calls add latency and cost to every dispatched
  task (mitigated by hints and small models).
- The capability index is a derived view that can go stale between
  applies.
- Task→agent assignment presumes sequential processing per agent.
  Parallel task consumption requires workload tracking (Phase 4).
- Open-ended `hints` map requires discipline around vocabulary
  growth.

## Relationship to existing ADRs

| ADR | Relationship |
|-----|-------------|
| 0001 | L3 is the home for queue + dispatcher. This ADR populates L3. |
| 0003 | Dispatcher calls `Runtime.launch()` or `Runtime.session()` to assign tasks. |
| 0004 | Ephemeral agents (Spot tier) spawned for unmatched tasks in Phase 4. |
| 0014 | Existing message endpoint still works for direct user→agent interaction. Tasks are a parallel path. |
| 0017 | Humans are `assigned_agent` values in the queue, once the human runtime exists. |
| 0018 | Pinned agents = default task targets. Spot agents = overflow/ephemeral. Open agents = iteration, not task targets (unless `hints.agent` names one). |
| 0020 | TaskQueue and Dispatcher are service plugin kinds (`queue`, `dispatcher`). SqliteTaskQueue and dispatcher-llm are bundled defaults. |
| 0022 | Every state transition defined here emits a `data-z:task:*` wire event. ADR 0022 is authoritative on wire shape; this ADR is authoritative on state semantics and queue storage. |

## Implementation notes

**New files:**

```
packages/core/src/contracts/queue.ts       ← TaskQueue contract
packages/core/src/contracts/dispatcher.ts  ← Dispatcher contract

packages/daemon/src/queue/
  sqlite-queue.ts        ← SqliteTaskQueue implementation
  queue-routes.ts        ← HTTP route handlers for /v1/tasks

packages/daemon/src/dispatcher/
  llm-dispatcher.ts      ← Dispatcher implementation (LLM call)
  capability-index.ts    ← builds index from fleet state
  dispatch-loop.ts       ← poll queue → dispatch → assign/requeue
  prompts.ts             ← LLM prompt templates for dispatching
```

**Daemon startup additions:**

1. `SqliteTaskQueue` created (new `queue.db` or table in `memory.db`),
   registered in `PluginRegistry` as `queue-sqlite`.
2. Capability index built from parsed fleet state.
3. Dispatch loop started (in-process, interval-based polling of queue).
4. Task routes mounted in API server.

**Existing code changes:**

- `api/server.ts` gains `/v1/tasks` route group.
- `fleet/executor.ts` (or reconciler, after the ADR 0003 refactor)
  triggers `index.rebuild()` after successful apply.
- `fleet/types.ts` already has `TriggerConfig` — cron/webhook triggers
  will feed `TaskQueue.enqueue()`.

**No changes to:** Runtime interface, fleet parser, existing agent
message flow (ADR 0014). The task queue is additive — a new ingress
path alongside the existing direct-messaging path.

## Next steps

- Validate this ADR with the team — especially the hints semantics and
  the Phase 1 scope (hint-only dispatch before LLM dispatcher).
- Implement Phase 1 (queue + direct assignment) as a thin slice.
- Spike the capability index builder against a real fleet YAML to
  verify the serialized format is useful to an LLM.
- Draft the dispatcher LLM prompt template — the prompt engineering
  here is load-bearing; get it right in a spike before the dispatch
  loop is wired.