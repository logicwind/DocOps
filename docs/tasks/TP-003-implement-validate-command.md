---
title: Implement `docops validate`
status: done
priority: p0
assignee: claude
requires: [ADR-0004, ADR-0006, ADR-0002]
depends_on: [TP-002]
---

# Implement `docops validate`

## Goal

Command that validates all DocOps files in a repo against the schemas and the graph invariants. Exit code 0 on pass, non-zero on fail. Designed for pre-commit hooks and CI.

## Acceptance

- Validates frontmatter for every file in `docs/context/`, `docs/decisions/`, `docs/tasks/`.
- Enforces graph invariants:
  - Tasks' `requires` must contain ≥1 valid reference (ADR-0004).
  - All references in `requires`, `supersedes`, `related`, `depends_on` point to existing documents.
  - No cycles in `supersedes` or `depends_on`.
  - Citations to superseded documents emit warnings (configurable as errors via `docops.yaml`).
- `--json` flag emits machine-readable report: `{files: [...], errors: [...], warnings: [...]}`.
- Supports `--only <path>` for single-file validation (useful in pre-commit).
- Clear human-readable error messages: file, line (when possible), rule violated, suggested fix.

## Notes

This command is a load-bearing gate. Every other command should be able to assume the repo is valid (or reject its work if not).

## Outcome (2026-04-22)

- **Packages:** `internal/loader` (walks `docs/` per `docops.yaml paths:`, returns a `DocSet` keyed by ID), `internal/validator` (orchestrates schema + graph checks, returns a `Report`). CLI wiring in `cmd/docops/cmd_validate.go` with a small dispatcher in `main.go`.
- **Schema checks** reuse TP-002's `internal/schema` — per-doc validation is pass 1.
- **Graph invariants** (ADR-0006) implemented:
  - Every `supersedes / related / requires / depends_on` ID must resolve to an existing doc.
  - Cycle detection via colored-DFS on `supersedes` and `depends_on`, deduplicated by canonical rotation so one cycle is reported once.
  - Citing a superseded CTX or ADR in a task's `requires:` is a warning by default; configurable via `docops.yaml gaps.task_requires_superseded_{adr,ctx}` to `error` or `off` (ADR-0008).
- **Wrong-directory detection:** a file whose filename prefix does not match its directory (e.g. `CTX-005` in `docs/decisions/`) is rejected at load time rather than silently skipped.
- **CLI flags:** `--json` emits `{ok, files, errors, warnings}`; `--only <path>` narrows reporting to a single file while still loading the full project so graph edges are resolvable. Exit codes: 0 pass, 1 validation failure, 2 bootstrap error.
- **Human output** is stable and scannable: `error path [rule] field: message`. Deterministic ordering via sort on path → rule → field.
- **Config:** extended `internal/config` with `Gaps` mirroring the `gaps:` block plus a `Severity` helper.
- **Tests:** 6 loader tests, 7 validator tests covering unresolved refs, dependency cycles, superseded-citation severity (warn vs error), schema-error surfacing, and determinism. `go test -race ./...` clean (35 tests total across the module).
- **E2E:** `docops validate` on this repo returns `0 errors, 0 warnings` across 30 documents. Negative smoke test in `/tmp/docops-fail-scenario/` produced the expected exit=1 with a clear `reference-unresolved` message.
- **Binary size:** 2.58 MB — 9% of the 30 MB budget. Plenty of headroom for TP-004/TP-005.
