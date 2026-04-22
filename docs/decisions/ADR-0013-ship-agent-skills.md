---
title: Ship /docops-* skills for agent tooling
status: accepted
coverage: required
date: 2026-04-22
supersedes: []
related: [ADR-0011, ADR-0014, ADR-0015]
tags: [distribution, agent-interface, skills]
---

# Ship /docops-* skills for agent tooling

## Context

Claude Code, Cursor, and similar agent tools have native concepts for "skills" or "slash commands" — markdown files that package a specific capability into a discoverable slash-command. GStack uses this pattern well (`/gstack-ceo`, `/gstack-eng-manager`, etc.). Requiring the user to type full CLI invocations for every DocOps action is slower than offering pre-packaged skill files.

## Decision

DocOps publishes a skill pack covering common workflows. Each skill is a markdown file that wraps CLI commands with agent-friendly instructions. Phase-1 skill set:

- `/docops-init` — scaffold DocOps in a repo.
- `/docops-state` — read `docs/STATE.md` and summarize for the agent.
- `/docops-audit` — run structural audit, surface gaps, offer to draft fixer tasks.
- `/docops-next` — show the next actionable task.
- `/docops-new-task` — interactive task creation with citation prompt.
- `/docops-new-adr` — interactive ADR creation.
- `/docops-new-ctx` — interactive CTX creation.
- `/docops-review <ADR-id>` — run semantic coverage review and write sidecar.
- `/docops-graph <id>` — show the typed reference graph around a document.

Skills are distributed two ways:

1. **As a standalone skill pack** (`@docops/skills-claude-code`, `@docops/skills-cursor`) installable per-IDE.
2. **Auto-scaffolded** into the repo on `docops init`, placed where Claude Code and Cursor expect them (`.claude/skills/` and `.cursor/commands/` respectively).

The skills are thin wrappers over the CLI — they exist for ergonomics, not for duplicated logic. A skill that gets out of sync with its CLI command is a bug.

## Rationale

- Matches existing agent-tooling conventions (GStack pattern).
- Lowers the friction of "what is the right command?" for agents and humans.
- Packaging as a skill pack means one `brew install` or one file copy gets a team productive.
- Auto-scaffolded skills mean repo-local customization is possible (projects can tune the skill prompts).

## Consequences

- Skills must stay in lockstep with CLI changes. CI should test that each skill's invocation produces the expected shape.
- New agent tools (future Cursor replacements, emerging IDEs) will need new skill ports. The CLI remains the canonical surface; skills are presentations.
- If a skill requires non-CLI logic (e.g., specific prompt wording for the agent), that logic lives in the skill file only. The CLI never absorbs agent-specific prompting.
