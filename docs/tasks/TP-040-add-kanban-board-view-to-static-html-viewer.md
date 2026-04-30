---
title: Add kanban board view to static HTML viewer
status: done
priority: p2
assignee: nix
requires: [ADR-0027]
depends_on: []
---

## Goal

A "Board" tab in the SPA that surfaces `docs/tasks/` as a kanban-style
column grid: `backlog` | `active` | `blocked` | `done`. Complements the
existing list/graph/timeline views — gives humans (and LLMs scanning the
same SPA) a fast visual read on work-in-flight at a glance.

Motivation: STATE.md `Active work` answers "what's open right now" in
prose; the sidebar lists tasks alphabetically without status grouping;
neither answers "what's the WIP shape across statuses" at a glance.

## Acceptance

- New `#/board` route + nav link.
- Four columns left-to-right: backlog, active, blocked, done. Column
  headers show count.
- `done` column collapses to last 10 by `last_touched`, with a "show all"
  toggle so the view doesn't drown in shipped work.
- Card content per task: ID linkified, title, priority chip, assignee
  if present, tag chips if present.
- Click card → navigate to task detail.
- Filter chips: priority (p1/p2/p3), tag.
- No new JS deps; pure CSS grid / flex.

## Out of scope

- Drag-to-move between columns (would need a write surface; the SPA is
  read-only by design per ADR-0027).
- Swim-lanes by assignee or sprint (no sprint concept in DocOps; revisit
  if cycles are added).

## Notes

Captured alongside TP-037 (timeline view) as part of the post-v0.6.0
viewer-enrichment pass.
