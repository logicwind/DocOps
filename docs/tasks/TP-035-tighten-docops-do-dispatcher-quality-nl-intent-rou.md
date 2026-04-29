---
title: Tighten /docops:do dispatcher quality — NL intent routing
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0029, ADR-0013]
depends_on: []
---

With ADR-0029, `/docops:do` becomes the primary user-facing free-form entry point. The granular slashes that previously gave users a fallback (e.g., `/docops:get`, `/docops:search`) are gone — so dispatcher quality directly determines whether the smaller surface feels more usable or just more limited.

## Goal

When a user types `/docops:do <natural-language intent>`, the right skill should fire >95% of the time on a representative test set, with no user retry needed.

## Scope

1. **Audit current `/docops:do` skill prompt** under `templates/skills/docops/do.md` — is the dispatch logic explicit, or does it lean on the agent's general routing? Make it explicit: include a small decision table (intent shape → skill name) inline.
2. **Build a fixture test set** of ~30 representative user intents covering each remaining granular skill: lookups, searches, listings, graph walks, doc creation (CTX/ADR/TP), close, audit, refresh, amend (post-ADR-0025), supersede, revise, baseline (post-ADR-0030).
3. **Score routing accuracy** by running each fixture through `/docops:do` and recording which skill the agent actually invokes. Fail any case below 95% and tune the dispatch table.
4. **Disambiguation patterns** — for ambiguous intents ("update ADR-9"), the dispatcher should ask one clarifying question (amend? supersede? revise scope?) rather than guess. Document this pattern in the skill.

## Acceptance criteria

1. `templates/skills/docops/do.md` contains an explicit intent → skill dispatch table.
2. A fixture file (`docs/.docops/test/do-routing-fixtures.json` or similar) lists ~30 intents with expected target skills.
3. Manual run-through of fixtures (or a scripted check, if the harness supports it) shows ≥95% correct dispatch.
4. The skill description (frontmatter `description:`) is tightened so it triggers on free-form intent phrasing without requiring the literal `/docops:do` slash — supports natural-conversation dispatch when the user doesn't type the slash.
5. A short "when to ask vs. when to act" rubric is included for ambiguous cases.

## Out of scope

- Adding new CLI verbs (separate tasks).
- Changing the underlying skill files this dispatcher routes to.

## Notes for the implementer

- Skill-creator best practices apply — keep granular skills' descriptions strong; the dispatcher works best when each target skill has a sharp trigger surface of its own.
- This is not a rewrite of `/docops:do`; it's a quality pass. Keep the prompt small.
