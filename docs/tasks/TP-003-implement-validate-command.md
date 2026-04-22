---
title: Implement `docops validate`
status: backlog
priority: p0
assignee: unassigned
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
