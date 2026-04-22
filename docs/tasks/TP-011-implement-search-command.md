---
title: Implement `docops search` — content + structured filter query
status: backlog
priority: p1
assignee: unassigned
requires: [ADR-0017, ADR-0002, ADR-0006]
depends_on: [TP-004]
---

# Implement `docops search` — content + structured filter query

## Goal

Ship a single command that answers "has this been discussed before?" — substring or regex match over title / tags / body, composable with structured frontmatter filters. Output is the focused slice an agent needs to pick its next read.

## Acceptance

- New subcommand `docops search <query> [flags]`.
- Text match:
  - Plain substring by default; `--regex` switches to `regexp`.
  - Case-insensitive by default; `--case` to opt into case-sensitive matching.
  - Empty query is valid when at least one structured filter is present (filter-only query).
- Structured filters (all AND-composed with the text match):
  - `--kind CTX|ADR|TP`
  - `--status <value>` (applies per-kind: ADR status, Task status; CTX has none — error if `--kind CTX --status ...`)
  - `--coverage required|not-needed` (ADR only — error on other kinds)
  - `--tag <value>` (ADR, repeatable; all tags must match)
  - `--priority <value>` (Task only)
  - `--assignee <value>` (Task only)
  - `--since <YYYY-MM-DD>` compared against `last_touched`
- Human output: one line per match with id, kind, title, and a ~120-char title-aware snippet around the match.
- `--json` output: array of `{id, path, kind, title, snippet, match_field}`.
- Ranking (ADR-0017 table): title > tags > body; first-paragraph body match ranks above later-body. Tie-break by `last_touched` desc, then `id` asc.
- Read frontmatter from the in-memory index (built like `docops index`). Read `.md` bodies lazily — only open files for matches that still need a snippet.
- Runs `docops validate` first and refuses on a broken repo (same pattern as `state` / `audit`).
- Exit code 0 always (no matches is not an error). Trailing human-mode summary: `N match(es)`.

## Notes

- Keep regex mode explicit; the default must remain predictable substring so agents can build queries without escaping.
- Body reads should respect frontmatter stripping (reuse `schema.SplitFrontmatter`) so matches don't trip on frontmatter text.
- Semantic search is explicitly deferred (ADR-0017). Do not introduce an embedding dep.
- Tests: each filter, text + filter interaction, ranking order, snippet shape, determinism, and a dog-food pass against the live repo.
