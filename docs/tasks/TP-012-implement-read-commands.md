---
title: Implement focused read commands — `get`, `list`, `graph`, `next`
status: backlog
priority: p1
assignee: unassigned
requires: [ADR-0018, ADR-0005, ADR-0006, ADR-0010]
depends_on: [TP-004]
---

# Implement focused read commands — `get`, `list`, `graph`, `next`

## Goal

Close the query-surface gap identified in ADR-0018 so agents never need to `cat docs/.index.json` to answer routine questions. Each command returns a focused slice of the index, not the whole graph.

## Acceptance

### `docops get <id> [--json]`

- Looks up one doc by ID (`ADR-0012`, `CTX-004`, `TP-003`).
- Returns the `IndexedDoc` for that ID: source frontmatter + derived fields (summary, word_count, last_touched, age_days) + reverse edges (superseded_by, referenced_by, derived_adrs for CTX, active_tasks, blocks, implementation for ADR, stale).
- Human output: a short, readable block; `--json` emits the raw `IndexedDoc`.
- Exit 0 on found, 1 on not-found (with clear error), 2 on bootstrap.

### `docops list [flags] [--json]`

- Lists docs, trimmed to the fields agents usually need: `id`, `kind`, `status` (per kind), `title`, `coverage` (ADR), `priority` (Task), `assignee` (Task), `last_touched`, `stale`, `implementation` (ADR).
- Filters (all AND):
  - `--kind CTX|ADR|TP`
  - `--status <value>` (per-kind semantics, same as search)
  - `--coverage required|not-needed` (ADR only)
  - `--tag <value>` (ADR)
  - `--stale` (only docs with `stale: true`)
  - `--since <YYYY-MM-DD>`
- Default sort: kind (CTX → ADR → TP), then id ascending.
- `--json` returns an array of trimmed records.

### `docops graph <id> [--depth N] [--json]`

- Emits the typed graph reachable from the starting doc out to depth N (default 1).
- Edges traversed: `supersedes`, `related`, `requires`, `depends_on`, plus their reverse edges (`superseded_by`, `referenced_by`, `blocks`, `active_tasks`, `derived_adrs`).
- Human output: an indented tree view. `--json` emits `{root, nodes: [...], edges: [{from, to, edge, direction}]}`.
- Cycle-safe — the validator already rejects bad cycles, but the walker must still guard against re-visits.

### `docops next [--assignee <name>] [--priority <p0|p1|p2>] [--json]`

- Picks one task to work on. Selection rules (first match wins):
  1. `status: active` for the given assignee (recent activity first).
  2. `status: backlog` with all `depends_on` targets in `status: done`.
  3. Within unblocked backlog: filter by `--assignee` and `--priority` if given.
  4. Break ties by priority (`p0` > `p1` > `p2`), then by id ascending.
- Human output: `TP-XXX (assignee, priority) title — requires: ...` plus a one-sentence reason (`active for you`, `unblocked and p0`, etc.).
- `--json` emits the full task record plus a `reason` field.
- Exit 0 on found, 1 on "no task matches" (so CI pipelines can branch), 2 on bootstrap.

### Shared constraints

- Run `validate` first in each command; refuse on a broken repo.
- Rebuild the index in-memory (consistent with `state` / `audit`).
- Deterministic output in both human and `--json` modes.
- Reuse existing struct types from `internal/index` wherever possible; trimmed views live in per-command types.

## Notes

These commands round out the query surface ADR-0018 commits to. Once they ship, the `templates/AGENTS.md.tmpl` guidance ("prefer CLI over raw .index.json") is backed by a complete implementation — no "you could use the CLI except for …" caveats.

Tests: per-command happy-path + error-path, filter composition, ranking in `next`, depth clamping in `graph`, dog-food pass.
