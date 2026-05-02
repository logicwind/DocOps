---
description: Ask DocOps which task to pick up next. Uses assignee, priority, status, and depends_on to recommend one task. Use at session start or after finishing a task.
---

# Cookbook: next

## Context
Pick one task to work on. The CLI ranks by descending priority, then
ascending ID, among tasks with no unmet `depends_on`. Non-zero exit
when nothing matches.

## Input
Optional filters: `--assignee <handle>`, `--priority <p0|p1|p2>`,
`--json`.

## Steps
1. Run:

   ```
   docops next
   docops next --assignee nachiket
   docops next --priority p0 --json
   ```

2. After selection, **read every doc in the chosen task's `requires:`
   and `depends_on:`** before writing code. This is non-negotiable —
   the citations encode the constraints the task must respect.

## Confirm
Chosen task ID + title, its `requires:` IDs, and what to read first. If
nothing matches, surface the message verbatim and suggest
`/docops:audit` or `/docops:new-task`.
