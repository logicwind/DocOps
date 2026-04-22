---
title: ADR `coverage` field — required vs. not-needed
status: accepted
coverage: required
date: 2026-04-22
supersedes: []
related: [ADR-0004, ADR-0008]
tags: [schema, coverage]
---

# ADR `coverage` field — required vs. not-needed

## Context

Some ADRs genuinely need no implementation tasks. Examples:
- Recording an external constraint ("we are required by law to do X").
- Documenting an already-finished decision retrospectively.
- Recording a choice that is pure policy ("we commit to using TypeScript in new services") — enforced by review, not by a task.

If DocOps treats every accepted ADR as a gap until tasks exist, these legitimate cases generate noise forever.

## Decision

Every ADR frontmatter includes a `coverage` field:

- `coverage: required` (default) — this ADR needs tasks to implement it. If accepted with zero citing tasks, it is a structural gap.
- `coverage: not-needed` — this ADR does not need tasks. Structural gap detection is suppressed. The body should briefly justify why.

## Rationale

- Explicit beats implicit. `not-needed` is a declaration the author makes consciously, visible in diff, reviewable in PR.
- The default `required` means the coverage expectation is on by default; opt-out is deliberate.
- Keeps the structural gap rules clean (ADR-0008) without exception logic.

## Consequences

- PR reviewers should push back on `coverage: not-needed` without a justification in the body.
- `docops audit --include-not-needed` flag can list these for periodic review — "is this still genuinely not needed?"
- Changing `coverage` from `not-needed` to `required` after the fact is fine; the gap will surface in the next index run.
- This field does not apply to CTX or tasks.
