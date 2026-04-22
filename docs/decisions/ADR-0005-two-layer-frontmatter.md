---
title: Two-layer frontmatter — source vs. computed index
status: accepted
coverage: required
date: 2026-04-22
supersedes: []
related: [ADR-0002, ADR-0006, ADR-0010]
tags: [schema, architecture, index]
---

# Two-layer frontmatter — source vs. computed index

## Context

Human-written frontmatter wants to be minimal (ADR-0002). Agents querying the repo want richer structure: reverse edges, resolved references, staleness, summaries, etc. Putting all of that into source frontmatter creates drift (humans forget to update `superseded_by`) and noise (fields readers don't care about).

## Decision

DocOps maintains two layers:

**Layer 1 — Source frontmatter**
- What humans and agents write into `.md` files.
- Minimal, stable, hand-authored.
- Only forward edges and essential state (see ADR-0002 for the final shape).

**Layer 2 — Indexed frontmatter (`docs/.index.json`)**
- Computed by `docops index` on every commit (and on-demand).
- Contains source fields **plus** derived fields (reverse edges, resolved refs, staleness, implementation status, etc.).
- Never hand-edited; the file is regenerated from source.
- Shipped as a single JSON document for efficient agent queries.

The CLI's read commands (`docops get`, `docops list`, `docops state`, `docops next`) return the enriched view. The source files remain clean for humans.

## Rationale

- Drift elimination. Fields that would become stale (reverse edges, implementation status) are computed, not reported.
- Context efficiency. An agent answering "what depends on ADR-0020?" reads one JSON file, not N markdown bodies.
- Diff-friendly source. `git diff` on `docs/` shows only genuine human edits, not churn.
- Single source of truth. Source files are canonical; index is a projection.

## Consequences

- Agents should prefer CLI read commands over raw filesystem reads when they want derived fields.
- `.index.json` is regenerated as a build step (via pre-commit hook or CI). Stale indexes are detected and the CLI warns.
- Whether `.index.json` is committed to git is a per-project choice documented in `docops.yaml`. Default: yes — committing it gives offline readers (including CI and new agents) immediate access without running `docops index` first.
- New derived fields can be added without any source-file migration.
