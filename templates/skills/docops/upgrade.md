---
name: upgrade
description: Refresh DocOps-owned scaffolding (skills, schemas, AGENTS.md block) in an existing project after `brew upgrade docops` (or equivalent). Preserves docops.yaml and the pre-commit hook by default.
---

# /docops:upgrade

Run after the docops binary itself has been upgraded — it pulls the new
binary's shipped templates into the current project without clobbering
config.

```
docops upgrade
```

Touches only DocOps-owned scaffolding:

- `.claude/commands/docops/*.md` and `.cursor/commands/docops/*.md` — synced to the shipped bundle (creates new files, refreshes changed ones, removes files that left the bundle).
- `.claude/skills/docops/*.md` — if this legacy folder exists from an older docops, its contents are removed (Claude Code reads slash commands from `.claude/commands/`, not `.claude/skills/`).
- `docs/.docops/schema/*.schema.json` — regenerated from `docops.yaml`.
- The `<!-- docops:start --> … <!-- docops:end -->` block inside `AGENTS.md` and `CLAUDE.md` — refreshed in place; content outside the markers is preserved. Either file is created if absent (so v0.1.x users gain CLAUDE.md on first upgrade).

Leaves alone by default:

- `docops.yaml`
- `.git/hooks/pre-commit`
- `docs/{context,decisions,tasks}/*.md`
- `docs/.index.json`, `docs/STATE.md`, `docs/.docops/counters.json`

Preview before writing:

```
docops upgrade --dry-run
```

Opt in to overwriting customizable bits:

```
docops upgrade --config    # also rewrite docops.yaml from the shipped template
docops upgrade --hook      # also reinstall .git/hooks/pre-commit
```

For CI or scripted use, skip the prompt and emit structured output:

```
docops upgrade --yes
docops upgrade --json
```

Exit codes: `0` on success or user abort; `2` when there is no
`docops.yaml` (run `docops init` first) or when the
`.claude/commands/docops/` directory contains user-added files docops did
not write.
