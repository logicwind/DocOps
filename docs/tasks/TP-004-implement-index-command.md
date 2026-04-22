---
title: Implement `docops index`
status: backlog
priority: p0
assignee: unassigned
requires: [ADR-0005, ADR-0006, ADR-0010]
depends_on: [TP-002, TP-003]
---

# Implement `docops index`

## Goal

Command that crawls the DocOps directories, computes the enriched graph, and writes `docs/.index.json`. The source of truth for every read command.

## Acceptance

- Runs `docops validate` first; aborts with clear error if the repo is invalid.
- Produces `.index.json` containing an array of documents, each with:
  - Source frontmatter fields.
  - Derived fields: `id`, `folder`, `path`, `summary` (first ~200 chars of body or explicit `summary:` override), `word_count`, `last_touched` (git log), `age_days`.
  - Reverse edges: `superseded_by`, `referenced_by: [{id, edge}]`, `derived_adrs` (for CTX), `blocks` (for tasks).
  - Computed `implementation` for ADRs (ADR-0010 rules).
  - Computed `stale` boolean using thresholds in `docops.yaml`.
- Deterministic output (stable key ordering) so diffs are meaningful.
- `--json` flag writes to stdout instead of file (for scripting).
- Fast: target < 200 ms on a 200-doc repo.

## Notes

Pre-commit hook should regenerate this. CI should fail if the committed `.index.json` does not match a fresh regeneration.
