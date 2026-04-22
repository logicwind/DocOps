---
title: Define frontmatter schemas for CTX, ADR, and Task
status: backlog
priority: p0
assignee: unassigned
requires: [ADR-0002, ADR-0006, ADR-0003]
depends_on: [TP-001]
---

# Define frontmatter schemas for CTX, ADR, and Task

## Goal

Implement strict typed schemas matching the final bare-minimum spec in ADR-0002. Output: reusable validators used by every other command and exported as JSON Schema for editor tooling.

## Acceptance

- Schemas for three doc types, exactly matching the field lists in ADR-0002.
- ID-reference regex enforced (`^(CTX|ADR|TP)-\d+$`).
- Tasks fail validation if `requires` is empty (ADR-0004 invariant).
- Enum fields reject unknown values (e.g., `status: in-review` on a task fails).
- `type:` on CTX is validated against the project's `docops.yaml` enum, not hardcoded.
- JSON Schema representation emitted for each type, written to `docs/.docops/schema/` on init.
- Unit tests covering: happy path per type, each failure mode, reference-format enforcement.

## Notes

Schema changes are an ADR-level event. Do not add fields without proposing an ADR.
