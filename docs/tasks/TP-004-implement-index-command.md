---
title: Implement `docops index`
status: done
priority: p0
assignee: claude
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

## Outcome (2026-04-22)

- **Package:** `internal/index` (`types.go`, `index.go`, `body.go`, `git.go`, `index_test.go`) plus `cmd/docops/cmd_index.go` CLI wrapper. Runs validate first and aborts with a clear message if the repo is invalid.
- **Source fields** carried forward verbatim; **derived fields** added: `id`, `kind`, `folder`, `path`, `summary` (first real paragraph after any heading lines, truncated to 200 chars, with `<!-- summary: ... -->` HTML-comment override), `word_count` (code fences stripped), `last_touched` (from `git log -1 --format=%cI`), `age_days`, `stale`.
- **Reverse edges** computed in a single pass: `superseded_by`, `referenced_by: [{id, edge}]`, `active_tasks` (ADR + CTX, filtered to `status: active`), `blocks` (Task), `derived_adrs` (CTX — via ADRs' `related` arrays plus a `\bCTX-\d+\b` body scan, deduplicated).
- **`implementation` for ADRs** implements the ADR-0010 truth table exactly: `not-needed → n/a`, zero citing tasks → `not-started`, all-done → `done`, any active → `in-progress`, mixed done+other → `partial`.
- **`stale`** per kind, keyed off `cfg.Gaps`: ADRs accepted+required with zero citing tasks past threshold, draft ADRs past threshold, active tasks past commit threshold, orphan CTX past threshold.
- **Determinism:** output sorted by ID, every reverse-edge slice sorted, struct field order drives JSON key order. Two consecutive runs produce byte-identical output (except the `generated_at` timestamp).
- **JSON shape note:** tasks use `task_status` to avoid a key collision with ADR's `status`. Consumers branch on `kind:` to interpret status fields correctly.
- **CLI contract:** `docops index` writes `cfg.Paths.Index` (default `docs/.index.json`); `--json` writes to stdout. Pre-index validate check exits 2 on failure with a clear message.
- **Verification:** 22 new tests in `internal/index` (all five implementation cases, reverse-edge types, stale scenarios with synthetic clock, body-summary edge cases, determinism). Dog-food smoke: `./bin/docops index` produces 30 docs in ~145 ms. Binary size 2.74 MB.
