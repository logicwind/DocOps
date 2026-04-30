---
title: Render amendment-aware UI in static viewer (ADR detail + Home recent)
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0027, ADR-0025]
depends_on: []
---

## Goal

Surface ADR-0025 amendments in the `docops html` / `docops serve` SPA — both
on the per-ADR detail page and on the Home view — sourced from
`recent_amendments` and per-doc `amendments` in the viewer bundle.

ADR-0027 §Consequences already calls this out: "Amendments rendering is
transparent — `internal/index.IndexedDoc` already has an `Amendments`
field; the SPA renders it if present, skips it if absent." This task closes
that loop.

## Acceptance

- `Bundle` carries `recent_amendments` (list from `index.RecentAmendments`)
  in addition to per-doc `amendments`.
- ADR detail view shows an "Amendments" section under the body when
  `doc.amendments` is non-empty: date, kind badge, by, summary, optional
  `ref` linkified, optional `affects_sections` chips.
- Home view shows a "Recent amendments" section after STATE.md when
  `bundle.recent_amendments` is non-empty (top 10, newest first).
- No new external dependencies; SPA stays a single file.

## Notes

Status: shipped under v0.6.0. This TP is a backfilled record of the work
so the index has the citation trail.
