---
name: do
description: Route a freeform natural-language intent to the right /docops:* skill. Use when the user knows what they want but doesn't know which command runs it.
---

# /docops:do

Interpret the user's request and dispatch to one matching skill.
Never do the work yourself — confirm the match, then hand off.

Routing table (intent → skill):

| Intent | Skill |
|---|---|
| "where are we", "what's the status", "summarise" | `/docops:progress` |
| "what's next", "pick a task" | `/docops:next` |
| "add a decision", "record that choice", "I need an ADR" | `/docops:new-adr` |
| "capture a PRD", "memo", "save constraints", "research note" | `/docops:new-ctx` |
| "add a task", "track this work", "create a TP" | `/docops:new-task` |
| "find X", "search for Y" | `/docops:search` |
| "look up ADR-0012" | `/docops:get` |
| "what depends on ADR-X", "blast radius" | `/docops:graph` |
| "list draft ADRs", "active tasks" | `/docops:list` |
| "what's broken", "coverage gaps" | `/docops:audit` |
| "finish TP-X", "mark done", "close this task" | `/docops:close` |
| "plan from CTX-X", "draft decision + tasks" | `/docops:plan` |
| "upgrade docops files", "refresh scaffolding" | `/docops:upgrade` |
| "validate", "reindex", "regenerate state" | `/docops:refresh` |

Match on intent, not literal wording. If two skills could fit, ask one
short clarifying question. If nothing matches, say so — do not invent a
command.
