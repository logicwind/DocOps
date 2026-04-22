# Braindump

Unstructured notes — ideas, half-baked thoughts, observations, things that
might matter later. Not part of the 3-bucket system (reference / roadmap /
archive). No rules about structure.

## Purpose

A scratchpad. Somewhere to write down a thought without deciding yet
whether it's an ADR, a roadmap item, or a dead end. Catches ideas that
would otherwise get lost between conversations.

## Rules (the few there are)

- **Date each entry.** Use `## YYYY-MM-DD — short title` so entries are
  searchable and orderable.
- **Lowest-friction format.** Bullets, fragments, one-liners all welcome.
  No need to be complete or coherent.
- **Graduation means removal.** When an entry becomes an ADR, a roadmap
  item, or is decisively rejected, remove it from here. Don't let the
  file grow forever — keep it a working edge, not a history.
- **Not for shipped work.** Shipped = archive. Active slice = roadmap.
  Decided = ADR. Braindump is strictly pre-structured thinking.

---

## Entries

<!-- Newest at the top. Prepend, don't append. -->

## 2026-04-17 — Graduated → ADR-0024 (apps/ vs packages/ layout)

Repo layout discussion: `apps/` (deployable end-user artifacts) vs
`packages/` (libraries imported by other packages). Covered by
ADR-0024. Entry removed per graduation rule.

## 2026-04-16 — Console consolidation graduated → ADR-0021

`test/harness/` killed; everything folded into `apps/console/` under
a Work / Admin / Debug nav grouping. Debug group gated on
`import.meta.env.DEV` + server-side 404 for `/debug/*` in prod. See
ADR-0021 for the full write-up and rationale.

## 2026-04-16 — Fleet-level features as USP

**Positioning context.** Zund shares the Pi agent core with OpenClaw and
Hermes Agent. Runtime parity means Zund cannot win on agent cleverness.
The defensible wedge is fleet-level state: **things that only make sense
when multiple agents + humans share a substrate.** OpenClaw isolates
workspaces; Hermes is single-user. Cross-agent coordination is the gap.

Reframed one-liner: *OpenClaw is a personal AI. Zund is a team AI
platform — agents and humans sharing memory, work, and cron as one
coordinated unit, with a promotion path from "one agent's skill" to
"whole fleet's capability" (see ADR 0018).*

**Feature backlog, ranked by wow÷effort:**

| Feature | Why it's fleet-only | Notes |
|---|---|---|
| Assets manager UI | One inbox of every artifact any agent produced — searchable, taggable, linkable to the session that made it. OpenClaw/Hermes can't do this (isolated workspaces). | First bet. Concrete, visible, immediately useful. |
| Fleet memory browser | Search across all facts saved by any agent. "What does the fleet know about customer X?" | Reuses existing `memory.db` + FTS5. Mostly UI work. |
| Capability graph | Who (agents + humans) can do what. Visual, queryable. Becomes the dispatcher's routing input. | Depends on ADR 0017 (humans) and L3 dispatcher. |
| Approval inbox | One queue for every pending human-gated decision across the fleet. | Depends on ADR 0017. |
| Cron calendar | All scheduled work across fleet in one view. Conflict detection, overload viz. | Depends on L3 triggers. |
| Cost + token dashboard | Per-agent, per-fleet, per-customer LLM spend. | Pro-tier feature. |
| Session timeline | Unified event stream, filterable by agent/human/capability/time. | Depends on zund://stream/v1 (ADR 0002). |
| Skills registry (fleet-curated) | One promoted set assigned via YAML — vs 8 agents drifting apart. | Tied to ADR 0018 Phase 2 promotion flow. |

**Sequencing instinct:** ship the assets manager first. It's the most
concrete and the most obvious "holy shit, OpenClaw can't do this" moment.
Memory browser second — cheap, high-signal. Everything else waits for
dispatcher/triggers (L3).

**Strategic implication:** don't treat OpenClaw/Hermes as competitors to
replace — treat them as runtimes to adopt (ADR 0018 Phase 2 + future
runtime-adapter ADR). Zund's coordination layer is the moat; their
ecosystem becomes leverage.

## 2026-04-16 — ADR index generator vs markdown formatter

`scripts/adr-index.ts` emits a plain markdown table. A post-commit
markdown formatter pads table columns for alignment. Result: every ADR
commit generates a clean table, gets formatted right after, and the
next commit shows a spurious "padding whitespace" diff.

Two fixes:
- Pre-pad the table output in the script to match what the formatter
  produces (idempotent output).
- Exclude `docs/reference/decisions/README.md` from the formatter.

Leaning toward pre-padding — keeps the formatter rule simple, means
the generated file matches whatever a human would produce manually.
Small addition: measure max width per column, pad each cell with
spaces, emit.
