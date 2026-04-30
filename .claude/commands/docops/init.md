---
name: init
description: Scaffold DocOps into a bare repository — creates docs/ folders, docops.yaml, JSON Schemas, AGENTS.md and CLAUDE.md blocks, and a pre-commit hook. Safe to run twice; use --dry-run to preview.
---

# Cookbook: init

## Context
First-time scaffolding into a repo (or specific directory). On a TTY,
init prints the plan and prompts `Proceed? [y/N]` before writing.
Non-TTY stdin and `--yes` skip the prompt; `--dry-run` also skips.

What it writes:
- `docs/{context,decisions,tasks}/` if absent.
- `docops.yaml` at the repo root with sensible defaults.
- `docs/.docops/schema/*.schema.json` for in-editor validation.
- `<!-- docops:start -->` block in `AGENTS.md` and `CLAUDE.md`.
- A language-agnostic `pre-commit` hook running `docops validate`.
- Slash commands into `.claude/commands/docops/` and
  `.cursor/commands/docops/`.

## Input
Optional positional `[dir]` (defaults to cwd). Optional flags:
`--dry-run`, `--force`, `--no-skills`, `--yes`.

## Steps
1. Run:

   ```
   docops init [dir]
   docops init --dry-run        # preview
   docops init --force          # overwrite drifted files
   docops init --no-skills      # skip skill scaffolding
   docops init --yes            # CI / non-interactive
   ```

2. After init, run `docops validate` to confirm everything parses, then
   `/docops:new-ctx` or `/docops:new-adr` to start capturing state.

## Confirm
List of files/folders written, whether the pre-commit hook landed, and
the next step (validate + first CTX/ADR).
