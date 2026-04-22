---
title: Define frontmatter schemas for CTX, ADR, and Task
status: done
priority: p0
assignee: claude
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

## Outcome (2026-04-22)

- **Packages:** `internal/schema` (types, parse, validate, jsonschema) and `internal/config` (docops.yaml loader).
- **Types** match ADR-0002 exactly (3 fields CTX, 7 fields ADR, 6 fields Task).
- **Strict YAML decode** — `yaml.v3` `KnownFields(true)` rejects typos and unknown fields rather than silently dropping them.
- **Multi-error reporting** — `ValidationErrors` collects every failure per doc so a user sees all problems at once.
- **ADR-0004 alignment** enforced two ways: tasks with empty `requires` fail; tasks whose `requires` list has no CTX or ADR (only TP refs) also fail.
- **JSON Schema** emitted for all three types. Context schema's `type` enum is wired to `docops.yaml.context_types`; Task schema uses the JSON Schema `contains` keyword to encode the ADR-0004 rule for IDE-side validation.
- **Cross-kind `related`** — refined during implementation: ADR `related:` accepts any ID kind (CTX/ADR/TP), since ADRs commonly cite the CTX that motivated them. Caught by the dog-food self-validation test against ADR-0014.
- **Self-validation** — `dogfood_test.go` runs the validator over every CTX/ADR/TP in `docs/` on every `go test`. All 30 live docs pass.
- **Tests:** 22 tests across `internal/config` and `internal/schema`. `go test -race ./...` clean.
- **Dependency added:** `gopkg.in/yaml.v3` (MIT, 1 transitive dep for its own tests).
