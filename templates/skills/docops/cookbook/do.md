---
name: do
description: Route a freeform natural-language intent to the right docops skill or CLI verb. Use when the user knows what they want but doesn't know which command runs it.
---

# Cookbook: do

## Context
Dispatch NL intent to one matching skill (or CLI verb when no skill
fits). **Never do the work yourself** — confirm the match, then hand off
to the cookbook chapter or CLI command.

The slash surface is small (`/docops:init`, `/docops:progress`,
`/docops:next`, `/docops:do`, `/docops:plan`); granular capabilities
live as cookbook chapters (NL-dispatched, no slash) and CLI verbs
(`docops <verb>`). Prefer the cookbook chapter; fall back to the CLI
verb only when no chapter exists.

## Input
A freeform user phrase.

## Steps
1. Match the phrase against the routing table:

   | Intent | Target |
   |---|---|
   | "where are we", "what's the status", "summarise" | `cookbook/progress.md` |
   | "what's next", "pick a task" | `cookbook/next.md` |
   | "plan from CTX-X", "draft decision + tasks" | `cookbook/plan.md` |
   | "add a decision", "I need an ADR" | `cookbook/new-adr.md` |
   | "capture a PRD", "memo", "research note" | `cookbook/new-ctx.md` |
   | "add a task", "track this work" | `cookbook/new-task.md` |
   | "find X", "search for Y" | `cookbook/search.md` |
   | "look up ADR-NNNN" | `cookbook/get.md` |
   | "what depends on ADR-X", "blast radius" | `cookbook/graph.md` |
   | "list draft ADRs", "active tasks" | `cookbook/list.md` |
   | "what's broken", "coverage gaps" | `cookbook/audit.md` |
   | "finish TP-X", "mark done" | `cookbook/close.md` |
   | "fix typo / dead link in ADR-X" | `cookbook/amend.md` |
   | "ADR-X is wrong, write a new one" | `cookbook/supersede.md` |
   | "narrow / widen ADR-X's scope" | `cookbook/revise.md` |
   | "upgrade docops files" | `cookbook/upgrade.md` |
   | "validate / reindex / regenerate state" | `cookbook/refresh.md` |

2. Match on intent, not literal wording. If two chapters could fit,
   ask one short clarifying question. If nothing matches, say so
   directly — do not invent a command.

## Confirm
The chapter chosen and a one-line restatement of the user's intent so
they can correct mismatched routing before the work runs.
