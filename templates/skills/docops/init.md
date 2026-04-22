---
name: init
description: Scaffold DocOps into a bare repository — creates docs/ folders, docops.yaml, JSON Schemas, an AGENTS.md block, and a pre-commit hook. Safe to run twice; use --dry-run to preview.
---

# /docops:init

Scaffold DocOps into this repository.

```
docops init [--dry-run] [--force] [--no-skills]
```

What it does:

- Creates `docs/context/`, `docs/decisions/`, `docs/tasks/` if absent.
- Writes `docops.yaml` at the repo root with sensible defaults.
- Writes JSON Schemas to `docs/.docops/schema/` for in-editor validation.
- Writes or refreshes the `<!-- docops:start -->` block inside `AGENTS.md`.
- Installs a language-agnostic pre-commit hook that runs `docops validate`.
- Scaffolds `/docops:*` skills into `.claude/skills/docops/` and `.cursor/commands/docops/`.

Flags:

- `--dry-run` — print what would change, write nothing.
- `--force` — overwrite files that have drifted from the shipped templates.
- `--no-skills` — skip scaffolding the agent skill files.

After init, run `docops validate` to confirm everything parses, then `/docops:new-ctx` / `/docops:new-adr` to start capturing the project's state.
