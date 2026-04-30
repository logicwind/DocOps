---
name: upgrade
description: Refresh DocOps-owned scaffolding (skills, schemas, AGENTS.md block) in an existing project after `brew upgrade docops` or equivalent. Preserves docops.yaml and the pre-commit hook by default.
---

# Cookbook: upgrade

## Context
Pulls the new binary's shipped templates into the current project
without clobbering config. Run after the docops binary is upgraded.

Touches:
- `.claude/commands/docops/*.md`, `.cursor/commands/docops/*.md` —
  synced to the shipped bundle (adds new, refreshes changed, removes
  files that left the bundle).
- `.codex/skills/docops/{SKILL.md, cookbook/*.md}` — skill bundle.
- `.claude/skills/docops/*.md` — legacy folder; cleaned up if present.
- `docs/.docops/schema/*.schema.json` — regenerated from `docops.yaml`.
- The `<!-- docops:start --> ... <!-- docops:end -->` block in
  `AGENTS.md` and `CLAUDE.md` — refreshed in place; outside-marker
  content is preserved.

Leaves alone by default: `docops.yaml`, `.git/hooks/pre-commit`,
`docs/{context,decisions,tasks}/*.md`, `docs/.index.json`,
`docs/STATE.md`, `docs/.docops/counters.json`.

## Input
None for the default run. Flags opt in to broader rewrites.

## Steps
1. Preview before writing:

   ```
   docops upgrade --dry-run
   ```

2. Run:

   ```
   docops upgrade
   docops upgrade --config    # also rewrite docops.yaml
   docops upgrade --hook      # also reinstall pre-commit hook
   docops upgrade --yes       # CI / non-interactive
   docops upgrade --json      # structured output
   ```

   Exit codes: `0` on success or user abort; `2` when there is no
   `docops.yaml` (run init first) or when the slash dir contains
   user-added files docops didn't write.

## Confirm
Files added, refreshed, removed (counts). Anything skipped because the
user opted out (`--config` / `--hook` not passed). Whether AGENTS.md and
CLAUDE.md blocks were refreshed.
