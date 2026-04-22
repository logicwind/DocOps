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

- `.claude/skills/docops/*.md` and `.cursor/commands/docops/*.md` — synced to the shipped bundle (creates new files, refreshes changed ones, removes files that left the bundle).
- `docs/.docops/schema/*.schema.json` — regenerated from `docops.yaml`.
- The `<!-- docops:start --> … <!-- docops:end -->` block inside `AGENTS.md` — refreshed in place; content outside the markers is preserved.

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
`docops.yaml` (run `docops init` first) or when the `.claude/skills/docops/`
directory contains user-added files docops did not write.
