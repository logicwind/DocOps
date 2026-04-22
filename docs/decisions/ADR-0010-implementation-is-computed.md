---
title: ADR `implementation` is computed, not source
status: accepted
coverage: required
date: 2026-04-22
supersedes: []
related: [ADR-0002, ADR-0005, ADR-0008]
tags: [schema, index, drift-prevention]
---

# ADR `implementation` is computed, not source

## Context

The Zund project's existing ADRs carry an `implementation:` field (values: `done | partial | in-progress | not-started`). In practice this field drifts — people forget to update it after merging a task, so an ADR stays `not-started` long after its work shipped. Self-reported state is the classic source-of-drift.

## Decision

`implementation` is removed from ADR source frontmatter and is instead computed by `docops index` from the graph:

| Condition | Computed `implementation` |
|---|---|
| `coverage: not-needed` | `n/a` |
| `coverage: required` AND zero citing tasks | `not-started` |
| all citing tasks `status: done` | `done` |
| any citing task `status: active` | `in-progress` |
| mixed: some `done`, some `backlog`/`blocked`, none `active` | `partial` |

Stored as a derived field in `.index.json`. Appears in `docops get`, `docops list --json`, and `STATE.md`.

## Rationale

- Cannot drift because it is not hand-edited.
- Makes the ADR → Task graph load-bearing (it actually informs state).
- Reinforces the alignment contract (ADR-0004): an ADR's implementation status is only non-trivial because tasks cite it.
- Removes a field from the source-schema audit surface (ADR-0002).

## Consequences

- Migrations from existing ADR systems (Nygard/MADR) need to convert `implementation` from source to derived. The migration guide will note this.
- Existing tools that expect to read `implementation` from the file must be pointed at `docops get <id> --json` instead.
- The derivation rules live in code and are documented. Edge cases (e.g., task citing the ADR but also citing a superseded CTX) need explicit handling; default behavior for now is to count the task as citing the ADR normally.
- If a project wants to pin `implementation` manually despite drift risk, they can add a `manual_implementation:` field in body YAML — but the index field is authoritative.
