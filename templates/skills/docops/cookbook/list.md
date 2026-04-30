---
name: list
description: List DocOps docs with optional filters (kind, status, coverage, tag, stale, since). Use when looking for a set of docs, not one — e.g. "all draft ADRs", "all active tasks".
---

# /docops:list

Enumerate docs with filters. Prefer this over reading `docs/.index.json`
directly.

```
docops list --kind ADR --status draft
docops list --kind TP --status active
docops list --kind ADR --coverage required
docops list --tag release
docops list --stale
docops list --since 2026-04-01
docops list --json
```

Filters compose (AND). `--status` semantics are per-kind:
- ADR: `draft`, `accepted`, `superseded`, `rejected`
- TP: `backlog`, `active`, `done`
- CTX: has no status; `--status` with `--kind CTX` is invalid

Default output is a table. Use `--json` for scripting.
