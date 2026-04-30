---
name: do
description: Route a freeform natural-language intent to the right docops skill or CLI verb. Use when the user knows what they want but doesn't know which command runs it.
---

# /docops:do

Interpret the user's request and dispatch to one matching skill (or, when no skill matches, the right `docops` CLI verb). Never do the work yourself — confirm the match, then hand off.

The slash-command surface is small (`/docops:init`, `/docops:progress`, `/docops:next`, `/docops:do`, `/docops:plan`); granular capabilities live as **skills** (NL-dispatched, no slash) and **CLI verbs** (`docops <verb>`). When dispatching, prefer the skill name; fall back to the CLI verb only when no skill exists.

Routing table (intent → target):

| Intent | Target |
|---|---|
| "where are we", "what's the status", "summarise" | `docops:progress` (skill + slash) |
| "what's next", "pick a task" | `docops:next` (skill + slash) |
| "plan from CTX-X", "draft decision + tasks" | `docops:plan` (skill + slash) |
| "add a decision", "record that choice", "I need an ADR" | `docops:new-adr` (skill) |
| "capture a PRD", "memo", "save constraints", "research note" | `docops:new-ctx` (skill) |
| "add a task", "track this work", "create a TP" | `docops:new-task` (skill) |
| "find X", "search for Y" | `docops:search` (skill) |
| "look up ADR-0012" | `docops:get` (skill) |
| "what depends on ADR-X", "blast radius" | `docops:graph` (skill) |
| "list draft ADRs", "active tasks" | `docops:list` (skill) |
| "what's broken", "coverage gaps" | `docops:audit` (skill) |
| "finish TP-X", "mark done", "close this task" | `docops:close` (skill) |
| "upgrade docops files", "refresh scaffolding" | `docops:upgrade` (skill) |
| "validate", "reindex", "regenerate state" | `docops:refresh` (skill) |

Match on intent, not literal wording. If two skills could fit, ask one
short clarifying question. If nothing matches, say so — do not invent a
command.
