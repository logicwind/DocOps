---
title: Ship /docops-* skill pack for agent tools
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0013]
depends_on: [TP-005, TP-006, TP-007, TP-008]
---

# Ship /docops-* skill pack for agent tools

## Goal

Produce the first version of the DocOps skill pack — markdown files that wrap CLI commands for Claude Code and Cursor so agents get discoverable slash-commands.

## Acceptance

- Claude Code skills in `packages/skills-claude-code/.claude/skills/`:
  - `docops-init`, `docops-state`, `docops-audit`, `docops-next`, `docops-new-task`, `docops-new-adr`, `docops-new-ctx`, `docops-review`, `docops-graph`.
- Cursor equivalents in `packages/skills-cursor/.cursor/commands/`.
- Each skill:
  - Uses the correct format for its IDE.
  - Invokes the standalone `docops` binary; does not call language-specific wrappers.
  - Is short (1–2 screens max) with clear intent and expected inputs.
- `docops init` auto-copies the appropriate skills to the repo unless `--no-skills` is set.
- Skills are tested: for each skill, a recorded agent session confirms it runs and returns the expected shape.
- CI lint across all skills to confirm they call only documented CLI flags.

## Notes

Skills are ergonomic wrappers, not new logic. If a skill wants to do something the CLI cannot, propose a CLI feature first — do not embed behavior in the skill that bypasses the CLI.

Future agent tools (Aider, Zed, Windsurf-specific) should each get a skill pack in `packages/skills-<tool>/`. Phase 1 covers Claude Code and Cursor only.
