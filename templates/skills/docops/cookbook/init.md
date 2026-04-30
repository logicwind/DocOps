---
name: init
description: Scaffold DocOps into a bare repository — creates docs/ folders, docops.yaml, JSON Schemas, AGENTS.md and CLAUDE.md blocks, and a pre-commit hook. Safe to run twice; use --dry-run to preview.
---

# /docops:init

Scaffold DocOps into this repository (or a specific directory).

```
docops init [dir] [--dry-run] [--force] [--no-skills] [--yes]
```

`[dir]` is optional. When given, init targets that directory (creating it if absent) instead of cwd.

What it does:

- Creates `docs/context/`, `docs/decisions/`, `docs/tasks/` if absent.
- Writes `docops.yaml` at the repo root with sensible defaults.
- Writes JSON Schemas to `docs/.docops/schema/` for in-editor validation.
- Writes or refreshes the `<!-- docops:start -->` block inside `AGENTS.md` and `CLAUDE.md` (both files share the same docops block; Claude Code reads CLAUDE.md by default while other agents read AGENTS.md).
- Installs a language-agnostic pre-commit hook that runs `docops validate`.
- Scaffolds `/docops:*` slash commands into `.claude/commands/docops/` and `.cursor/commands/docops/`.

Flags:

- `--dry-run` — print what would change, write nothing.
- `--force` — overwrite files that have drifted from the shipped templates.
- `--no-skills` — skip scaffolding the agent skill files.
- `--yes` / `-y` — skip the interactive confirm prompt (required in CI and scripts).

On a TTY, init prints the plan and prompts `Proceed? [y/N]` before writing. Non-TTY stdin (CI, pipes) and `--yes` both skip the prompt. `--dry-run` also skips the prompt.

After init, run `docops validate` to confirm everything parses, then `/docops:new-ctx` / `/docops:new-adr` to start capturing the project's state.
