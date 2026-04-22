---
title: Implement `docops init` — scaffold DocOps in a repo
status: done
priority: p1
assignee: unassigned
requires: [ADR-0001, ADR-0015, ADR-0013, ADR-0016]
depends_on: [TP-002]
---

# Implement `docops init` — scaffold DocOps in a repo

## Goal

Command that turns a bare repository into a DocOps-enabled one. Idempotent — safe to run twice.

## Acceptance

- Creates `docs/context/`, `docs/decisions/`, `docs/tasks/` if absent.
- Writes `docops.yaml` at repo root with sensible defaults (context_types, thresholds, enums).
- Writes `docs/.docops/schema/*.json` (JSON Schema files emitted from TP-002 output).
- Reads template content from `templates/` in the DocOps source (per ADR-0016) rather than carrying it inline.
- Writes or updates `AGENTS.md` from `templates/AGENTS.md.tmpl` (ADR-0015). Delimited with HTML comments; preserves existing content outside the block.
- Writes `docops.yaml` from `templates/docops.yaml.tmpl` if absent.
- Installs a pre-commit hook runner (lefthook config + `.lefthook.yml`, or an equivalent non-Node-dependent option; must not require the user to have Node or Bun).
- Auto-scaffolds agent skills into `.claude/skills/` and `.cursor/commands/` (ADR-0013).
- Prints a "next steps" summary: `docops new adr`, `docops validate`, etc.
- `--force` flag overwrites existing scaffolded files that have drifted.
- `--dry-run` flag shows what would change without writing.

## Notes

Init is a user's first experience with DocOps. It must succeed on a brand-new repo with only a `.git` folder.

Pre-commit hook distribution is a design decision inside this task: options include bundling a small shim binary, documenting `pre-commit.com` integration, or requiring users to wire it manually. Pick whichever keeps the language-agnostic promise (ADR-0012).
