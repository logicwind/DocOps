---
title: Command-surface tiering — moments, verbs, operations
status: accepted
coverage: required
date: "2026-04-27"
supersedes: []
related: [ADR-0013, ADR-0018, ADR-0028, ADR-0025]
tags: [command-surface, slash-commands, skills, agent-interface]
---

## Context

ADR-0028 standardized `/docops:*` as **slash commands** delivered per-harness, replacing the earlier skill-as-command model from ADR-0013. As of v0.5.x, ~17 slash commands exist — one per CLI verb (`get`, `list`, `search`, `graph`, `audit`, `validate`, `index`, `state`, `refresh`, `new-ctx`, `new-adr`, `new-task`, `close`, `next`, `progress`, `do`, `plan`, `init`, `upgrade`).

This 1:1 mirroring made sense as a discoverability move when DocOps was new, but it conflates three distinct audiences:

1. **The user** — types slash commands at session-start or milestone moments. Cognitive budget is small (≤ ~6 commands they'll actually remember).
2. **The LLM agent** — dispatches capabilities from natural-language intent. Needs *granular* triggers for accuracy (per skill-creator best practices).
3. **The shell / scripts** — the CLI is the underlying API; should stay complete and orthogonal (per ADR-0018, "read commands are the query API").

Treating these as one surface optimizes for none. Granular commands like `/docops:get` or `/docops:validate` are LLM territory — a user who knows enough to type `/docops:get ADR-9` would be faster typing `docops get ADR-9` directly. Conversely, the user-facing milestones (`progress`, `next`, `plan`, `baseline`) deserve dedicated, prominent surfaces.

## Decision

Tier the surface by audience:

| Tier | Audience | Cardinality | Examples |
|---|---|---|---|
| **Slash commands** (`/docops:*`) | User types | ~6, milestone moments | `init`, `progress`, `next`, `do`, `plan`, `baseline` |
| **Skills** | LLM dispatches from NL intent | Many, granular | `get`, `list`, `search`, `graph`, `new-ctx`, `new-adr`, `new-task`, `amend`, `supersede`, `revise`, `close`, `audit`, `refresh`, `state` |
| **CLI** (`docops ...`) | LLM (via Bash) + shell scripts | Complete + orthogonal | every verb |

Skills and slash commands are no longer 1:1. A skill exists for every granular capability (so LLM dispatch stays accurate); a slash command exists only for the handful of user-typed *moments*.

`/docops:do` becomes the free-form dispatcher: the user describes intent ("close TP-024", "show me what changed since v1") and routing happens through skill selection, not through the user remembering the right slash.

## Rationale

- **Different audiences need different shapes.** Users speak in moments ("where are we?"); LLMs speak in verbs ("get ADR-9"); CLI speaks in operations. Forcing all three through one surface optimizes for none.
- **Skill triggering accuracy depends on granularity** — a single mega-skill loses dispatch precision. Keeping the skill set granular while shrinking the slash set is the only way to get both.
- **Cognitive load is the bottleneck for adoption.** A new DocOps user shouldn't have to learn 17 slashes before being productive; they need 6 moments and the confidence that the LLM will route the rest.
- **Reverses ADR-0028's over-extension, not its principle.** ADR-0028 was right to deliver slash commands per-harness; this ADR narrows *which* commands deserve that treatment.

## Consequences

- Most current `/docops:*` slash commands are deprecated (kept for one minor version for muscle memory, then removed). The underlying skills and CLI verbs remain.
- `docops upgrade` must clean up deprecated slash files in `.claude/commands/docops/`, `.cursor/commands/docops/`, `.opencode/command/`, `.codex/skills/docops/` so users on old versions don't keep stale entries.
- `/docops:do` becomes load-bearing — its dispatcher quality determines whether the smaller slash surface feels more usable or just more limited.
- Skill descriptions need a pass to ensure NL triggering still routes correctly without a slash to fall back on.
- Pairs cleanly with ADR-0025 (amendments) and the forthcoming baseline ADR — both add CLI verbs + skills, but neither needs a new slash command.
