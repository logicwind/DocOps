---
title: Add timeline view to static HTML viewer
status: done
priority: p2
assignee: nix
requires: [ADR-0027]
depends_on: []
---

## Goal

A chronological "Timeline" tab in the SPA that surfaces project events on
a single time axis: ADR transitions, amendments, task closes. Helps humans
and LLMs reason about *when* decisions and work happened — orthogonal to
the existing kind-grouped sidebar and graph views.

Motivation: amendments + named-baselines work (ADR-0030 draft) make
"what changed when" a recurring question. STATE.md answers "recently"
narrowly; the graph answers "what relates"; neither answers "show me the
last 3 months."

## Acceptance

- New `#/timeline` route + nav link.
- Vertical time axis grouped by month, newest first.
- Event sources from existing bundle data (no schema changes for v1):
  - ADR `date` (accepted_at-ish — current `date:` field).
  - Each entry in `doc.amendments[]` (per-ADR).
  - Task `last_touched` for tasks with `task_status: done`.
  - Optionally CTX `last_touched` for context adds.
- Event row: kind icon, ID linkified, short title, optional badge
  (kind for amendments, status for ADR/TP).
- Filter chips: kind (CTX/ADR/TP/amendment), tag (if present).
- Click row → navigate to doc detail.
- No new JS deps; pure CSS grid / SVG.

## Out of scope

- Git-history-derived events (first-seen-at, status-change-at). The index
  doesn't carry these today; if we want them later it's a separate TP that
  walks `git log` per doc at index time.
- Interactive zoom / brushing.

## Notes

Captured as a follow-up to the v0.6.0 release. Pre-release we shipped
amendment rendering on ADR detail + Home; timeline is the natural next
view.
