---
title: Shrink /docops:* slash surface to 6 moments + deprecation pass in upgrader
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0029, ADR-0028]
depends_on: []
---

Implement the slash-command tiering decision from ADR-0029. The user-facing slash surface drops from ~17 to ~6 milestone-moment commands; the rest become skill-only.

## Keep as slash commands (~6, milestone moments)

- `/docops:init`
- `/docops:progress`
- `/docops:next`
- `/docops:do`
- `/docops:plan`
- `/docops:baseline` *(once ADR-0030 ships; new addition)*

## Deprecate as slash commands (move to skill-only)

`get`, `list`, `search`, `graph`, `audit`, `validate`, `index`, `state`, `refresh`, `new-ctx`, `new-adr`, `new-task`, `close`, `upgrade`.

The skills under `templates/skills/docops/` stay — natural-language dispatch still routes here. Only the harness-specific slash files are removed.

## Acceptance criteria

1. `templates/skills/docops/` retains skill files for every granular capability (no skills deleted).
2. The per-harness slash command emitters (Claude Code `.claude/commands/docops/`, Cursor `.cursor/commands/docops/`, OpenCode `.opencode/command/docops-*`, Codex `.codex/skills/docops/` per ADR-0028) emit only the 6-moment set for fresh installs.
3. `docops upgrade` performs a **deprecation pass**: for each known-deprecated slash filename, delete it from the user's harness folders if present *and* unmodified vs. the previous version's template (use the existing template-hash check from ADR-0021). Modified files are left in place with a one-line warning so the user can clean up manually.
4. The deprecation list is data-driven — a `deprecated_slashes:` block in `docops.yaml` (or an internal constant) so future tier changes are configurable, not code-edited.
5. README + `/docops` index doc updated to document the 6 moments and link to skills for the rest.
6. Smoke test: in a fresh repo, `docops init` produces 6 slash files per harness; in a v0.5.x repo, `docops upgrade` removes deprecated unmodified files and reports a count.

## Out of scope

- Renaming or restructuring skill files.
- Behavior changes to underlying CLI verbs.
- The `/docops:do` dispatcher quality (tracked separately — load-bearing under ADR-0029).

## Notes for the implementer

- Look at TP-023 (commit `379ee51`) for how the original skills→slash move was done; the deprecation logic is the inverse path.
- Test on Claude, Cursor, OpenCode, Codex layouts — TP-033 added Codex's collapsed skill-bundle layout.
- `docops upgrade` already preserves user edits; reuse that pattern.
