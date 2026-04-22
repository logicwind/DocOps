---
id: "0017"
title: Humans as fleet members
date: 2026-04-16
status: draft
implementation: not-started
supersedes: []
superseded_by: null
related: ["0002", "0003", "0014"]
tags: [architecture, humans, dispatcher, l3]
---

# 0017 · Humans as fleet members

Date: 2026-04-16
Status: draft
Related: ADR 0003 (Runtime interface), ADR 0014 (message endpoint), ADR 0002 (stream protocol)

## Context

Zund's fleet model today assumes every node in the fleet is an agent —
something the daemon launches, messages, and reaps. But the interesting
coordination patterns in a real team mix agents and humans:

- A research agent drafts a brief, a *human* reviews and approves, a writer
  agent expands it.
- A cron trigger fires at 9am. If the task is "draft the weekly report," it
  dispatches to the writer agent. If the task is "sign off on the deploy,"
  it dispatches to Nachiket on Telegram.
- An agent hits a gray-area decision and needs a human judgment call before
  proceeding — the dispatcher should treat "ask a human" as just another
  capability in the fleet, not a hardcoded escape hatch.

Competing products treat humans as *users of* the agent (Hermes talks to
you from Telegram; Claude Code runs in your terminal). Zund's shared
substrate — fleet-level memory, cron, dispatch, capability index — makes a
stronger pattern possible: humans as *members of* the fleet, addressable by
the same dispatcher that picks agents.

This is not "human-in-the-loop" as an agent feature. It's "human as a kind
of runtime."

## Decision (draft)

Treat humans as a first-class fleet member kind. A human is declared in the
fleet folder the same way an agent is, and participates in the same
capability index, dispatcher, cron, memory, and event stream.

```yaml
# fleet/humans/nachiket.yaml
kind: Human
name: nachiket
capabilities:
  - code-review
  - deploy-approval
  - business-decisions
channels:
  - telegram: "@nachiket"
  - email: patel.nachiket.r@gmail.com
availability:
  timezone: Asia/Kolkata
  hours: "09:00-19:00"
  escalation: after 2h no-response → fallback-agent
```

The Runtime interface (ADR 0003) gains a `human` runtime alongside
`pi`/`vm`/`ssh`. The human runtime's `launch` is a no-op; `message` routes
through a channel (Telegram, Slack, email) and resolves when the human
replies; `events()` emits `human.prompted`, `human.responded`,
`human.timeout`.

From the dispatcher's point of view, asking Nachiket for a code review is
the same shape of operation as asking the writer agent for a draft —
capability match, dispatch, await result, record in session store.

## What this unlocks

- **Cron dispatches to humans.** "Every Friday at 5pm, ask Nachiket for a
  weekly retro" is a fleet resource, not a separate reminder app.
- **Agents invoke humans as tools.** A research agent needing a factual
  judgment call emits a dispatch; Nachiket gets a Telegram ping; the
  response flows back as a tool result.
- **Shared memory spans humans and agents.** Nachiket's Telegram response
  lands in the fleet memory store the same way an agent's tool output does.
  The writer agent reads it tomorrow.
- **Availability-aware routing.** Dispatcher respects human timezones and
  escalation rules — if nobody's online, fall back to an agent with a
  flagged assumption.
- **Audit trail is uniform.** Event stream shows "agent → human → agent"
  handoffs as one session, not three disconnected systems.

## Open questions

- **Identity and auth.** How does Zund verify the reply on Telegram is
  actually from Nachiket? Pairing flow at `zund apply` time, similar to
  Hermes's DM pairing.
- **Capability vocabulary.** Humans and agents share a capability namespace
  or separate? Probably shared — the dispatcher shouldn't care who provides
  `code-review`.
- **SLA semantics.** Agents respond in seconds; humans respond in hours or
  never. Task queue and stream protocol need timeout/retry/escalation
  primitives.
- **Privacy.** Does a human's Telegram reply go into fleet-level memory by
  default? Probably opt-in per-capability or per-human.
- **Pricing/licensing.** Humans-in-fleet feels like a commercial-tier
  feature (team-scale, audit-grade). OSS might ship single-human; Pro adds
  multi-human with RBAC.

## Consequences

**Makes easier:**

- Collapses "approval workflows," "on-call paging," "daily standups with
  bots" into one fleet primitive.
- Strengthens Zund's "team AI platform" thesis — humans as team members,
  not as users — in a way Hermes/Claude Code structurally can't.
- Reuses the dispatcher, capability index, memory store, and event stream
  that already need to exist for multi-agent routing. Minimal net-new
  infrastructure.

**Makes harder:**

- The Runtime interface must handle async, long-lived, unreliable responders
  (humans are the worst runtime). Forces timeout/escalation primitives into
  L3 earlier than a pure-agent design would need.
- Messaging gateway surface (Telegram/Slack/email bridges) becomes a
  requirement, not an optional extra. Large scope addition.
- Consent, identity, and privacy become product concerns from day one.

## Next steps

- Park here until L3 dispatcher design firms up (see `roadmap/next.md`).
- Revisit after the first real multi-agent fleet is running — the shape of
  the human primitive should be informed by which coordination patterns
  actually show up in practice.
- If promoted from draft, split into: (a) `Human` resource kind in the
  fleet schema, (b) messaging gateway ADR, (c) availability/escalation
  protocol ADR.
