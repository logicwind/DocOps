---
title: Typed edges with computed reverse edges
status: accepted
coverage: required
date: 2026-04-22
supersedes: []
related: [ADR-0005]
tags: [schema, graph, index]
---

# Typed edges with computed reverse edges

## Context

ADR tools conventionally have one edge type: `supersedes`. That is not enough to express the relationships a real project has — tasks relate to decisions differently than decisions relate to each other. Meanwhile, expressing edges in both directions (forward AND reverse) in source frontmatter creates drift: an author updates `supersedes` but forgets `superseded_by` on the other side.

## Decision

DocOps uses three typed edges, all directional, written only in the forward direction by authors:

- **`supersedes: [id, ...]`** — this doc replaces another. Applies to ADR → ADR and CTX → CTX.
- **`related: [id, ...]`** — this doc is thematically connected to another. Applies to ADR → ADR, ADR → CTX, CTX → CTX. Non-authoritative; useful as a soft cluster hint.
- **`requires: [id, ...]`** — this task depends on / must respect a decision or stakeholder input. Applies to Task → ADR, Task → CTX. Mandatory for tasks (ADR-0004).

A fourth edge, **`depends_on: [id, ...]`**, applies to Task → Task for sequencing.

Reverse edges are never written by authors. `docops index` computes and stores them in `.index.json`:

- `superseded_by` (reverse of `supersedes`)
- `referenced_by: [{id, edge}]` (reverse of all forward edges)
- `active_tasks` (reverse of `requires` filtered to `status: active`)
- `derived_adrs` (ADRs whose `related`/body cite a CTX)
- `blocks` (reverse of `depends_on`)

## Rationale

- Forward-only authoring removes an entire class of drift errors.
- Three edge types is the minimum that can express: "this replaced that," "these are adjacent," "this justifies that." Fewer edges collapse into loose links (Markplane); more edges overwhelm authors.
- Computed reverse edges save agent context: answering "what depends on ADR-0020?" is one query against the index instead of grepping every task file.

## Consequences

- Validator rejects a `supersedes` or `related` edge to a document that does not exist.
- Cycles in `supersedes` are errors. Cycles in `related` are permitted (it is symmetric).
- Cycles in `depends_on` are errors.
- Edges to superseded docs are warnings (configurable as errors).
- Adding a fourth edge type requires an ADR.
