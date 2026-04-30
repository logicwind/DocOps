---
name: search
description: Substring or regex search across DocOps doc titles, tags, and bodies, with structured filters (kind, status, coverage, tag, priority, assignee, since). Use when looking for docs by content, not ID.
---

# Cookbook: search

## Context
Text + frontmatter query. Default match is case-insensitive substring;
add `--regex` for regular expressions, `--case` for case-sensitive.
Filters narrow before text match — `--kind TP --status active "foo"`
is cheaper than searching then filtering.

Semantic / embedding search is out of scope — use substring + filters,
or have the user point at specific IDs via `/docops:get`.

## Input
A query string and/or any of: `--kind`, `--status`, `--coverage`,
`--tag`, `--priority`, `--assignee`, `--since`. Query is optional when
at least one filter flag is present.

## Steps
1. Run:

   ```
   docops search "release process"
   docops search "homebrew" --kind ADR
   docops search --kind TP --status active --priority p1
   docops search "upgrade" --regex --case
   docops search "migration" --json
   ```

## Confirm
Hit count and the matched IDs (or table). If 0 hits, say so directly.
