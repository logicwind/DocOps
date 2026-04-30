---
name: search
description: Substring or regex search across DocOps doc titles, tags, and bodies, with structured filters (kind, status, coverage, tag, priority, assignee, since). Use when looking for docs by content, not ID.
---

# /docops:search

Query docs by text and/or frontmatter.

```
docops search "release process"
docops search "homebrew" --kind ADR
docops search --kind TP --status active --priority p1
docops search "upgrade" --regex --case
docops search --tag release --since 2026-04-01
docops search "migration" --json
```

The query is optional when at least one filter flag is present. Default
match is case-insensitive substring; add `--regex` for regular
expressions and `--case` for case-sensitive matching.

Filters narrow before text match, so `--kind TP --status active "foo"`
is cheaper than searching and then filtering.

Semantic/embedding search is out of scope — use substring + filters, or
let the user point at specific IDs via `/docops:get`.
