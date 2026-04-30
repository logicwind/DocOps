---
title: Annotate graph nodes — amended ADRs, draft, stale
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0027, ADR-0025]
depends_on: []
---

## Goal

Enrich the Cytoscape graph in the SPA with per-node visual cues drawn from
data already in the bundle:

- ADRs with ≥1 amendment get a small badge (e.g. `★N` count) or a thicker
  border in the amendment color (amber/300, matching the detail-view UI).
- ADRs in `draft` status render with a dashed border to disambiguate from
  accepted nodes at a glance.
- Any doc with `stale: true` (already computed by the indexer) gets a
  muted/grey overlay.

Motivation: today the graph only encodes kind (column) and edge type
(color). Status and amendment density are useful at-a-glance signals when
scanning a 50+ ADR project for "what's hot."

## Acceptance

- Badge or border treatment for ADRs with amendments; count derived from
  `doc.amendments.length`.
- Dashed border or distinct fill for `status: draft` ADRs.
- Stale-doc treatment honors `doc.stale`.
- Legend updated to document the new cues.
- No new dependencies; Cytoscape style rules only.

## Out of scope

- Edge styling beyond what already ships (color by edge type).
- Filter/toggle UI for these annotations.
