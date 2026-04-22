---
title: Content search — substring + structured filters; semantic deferred
status: accepted
coverage: required
date: 2026-04-22
supersedes: []
related: [ADR-0005, ADR-0006, ADR-0011, ADR-0018]
tags: [cli, query, search, ux]
---

# Content search — substring + structured filters; semantic deferred

## Context

Agents working in a DocOps repo routinely ask "has this been discussed before?", "what did we decide about rate limiting?", "are there any tasks touching auth?". The current command set (`validate`, `index`, `state`, `audit`) covers structural and graph-shaped queries but has no way to answer content-shaped ones. Agents fall back to `grep` or, worse, guess from filenames — which loses frontmatter context (kind, status, coverage, tags) and returns no ranking signal.

Without a first-class search, the typed substrate promise (ADR-0001, ADR-0014) is incomplete: we have structure, but the content of 30–3000 markdown documents is not queryable through the tool.

## Decision

Phase 1 adds `docops search` with two kinds of filters composed with AND:

1. **Text match** over title, tags, and body. Plain substring by default; `--regex` flag for `regexp` package patterns. Case-insensitive by default; `--case` flag to opt into case-sensitivity.
2. **Structured filters** from frontmatter:
   - `--kind CTX|ADR|TP`
   - `--status <value>` (applies to the kind's status field — ADR `status`, Task `status`; CTX has none)
   - `--coverage required|not-needed` (ADR only)
   - `--tag <value>` (ADR, repeatable for AND)
   - `--priority <value>` (Task)
   - `--assignee <value>` (Task)
   - `--since <YYYY-MM-DD>` on `last_touched`

Output per match: `{id, path, kind, title, snippet, match_field}` — enough for an agent to decide which 2–3 docs to load next without loading the full body of each.

Ranking is simple and deterministic:

| Rank | Reason |
|---|---|
| 1 | match in `title` |
| 2 | match in `tags` |
| 3 | match in `body` (first-paragraph match > later-body match) |

Ties broken by `last_touched` descending, then `id` ascending.

CLI contract:

- `docops search <query> [filters]` — human output (one line per match, title-aware snippet truncated to ~120 chars).
- `docops search <query> --json` — structured output for agent scripting.
- `docops search --kind ADR --coverage required --status accepted` — filter-only query (empty text match).
- Reads the index for filter fields; reads `.md` bodies from disk only for matches that need a snippet.
- Exit 0 always (no matches is not an error); a summary `N match(es)` appears at the end of human output.

## What is deferred

- **Semantic / embedding search.** Introducing an embedding model as a runtime dep or cloud call breaks ADR-0012 (zero-runtime binary) and ADR-0011 (CLI-first, no MCP in phase 1). Revisit only if field feedback shows substring + filter cannot satisfy common queries.
- **Full-text ranking (BM25, TF-IDF).** The rank table above is intentionally simple and transparent. BM25 buys real relevance at scale but needs tokenisation choices we don't want to freeze yet.
- **Cross-repo search.** Out of scope — a single binary per repo is the contract.

## Rationale

- Fills an obvious functional hole without contradicting existing ADRs.
- Substring + structured filter covers 80%+ of the real queries we can anticipate from agent sessions.
- Output shape (`id`, `path`, `snippet`, `match_field`) is already what every other agent-facing command emits — consistent UX.
- No new third-party deps (regexp + the existing YAML/markdown surface suffice).

## Consequences

- `docops search` becomes part of the phase-1 feature set; task TP-011 implements it.
- The command must stay under the existing 30 MB binary budget — regexp is stdlib so this is free.
- Future semantic search is a phase-2 ADR; its output shape should remain compatible with this one.
- Agents should learn to reach for `docops search` before `grep` — the user-facing AGENTS template is updated to say so.
