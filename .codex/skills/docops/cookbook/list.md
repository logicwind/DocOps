---
description: List DocOps docs with optional filters (kind, status, coverage, tag, stale, since). Use when looking for a set of docs, not one — e.g. "all draft ADRs", "all active tasks".
---

# Cookbook: list

## Context
Enumerate docs with filters. Filters compose (AND). Prefer this over
reading `docs/.index.json` directly.

## Input
Any combination of: `--kind`, `--status`, `--coverage`, `--tag`,
`--stale`, `--since`.

`--status` semantics are per-kind:
- ADR: `draft` | `accepted` | `superseded`
- TP: `backlog` | `active` | `blocked` | `done`
- CTX: has no status; `--status --kind CTX` is invalid.

## Steps
1. Run:

   ```
   docops list --kind ADR --status draft
   docops list --kind TP --status active
   docops list --tag release --since 2026-04-01
   docops list --json
   ```

   Default output is a table; `--json` for scripting.

## Confirm
Hit count and the table (or filtered set). If 0 hits, say so directly.
