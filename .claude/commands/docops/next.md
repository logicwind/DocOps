---
name: next
description: Ask DocOps which task to pick up next. Uses assignee, priority, status, and depends_on to recommend one task. Use at session start or after finishing a task.
---

# /docops:next

Find the next task to work on.

```
docops next
```

Filter when the project has multiple contributors or priority bands:

```
docops next --assignee nachiket
docops next --priority p0
docops next --json
```

The CLI picks one task by descending priority (p0 → p2), then ascending
ID among tasks with no unmet `depends_on`. If no task matches, exit code
is non-zero and stderr says `no task matches`.

After selecting a task, read every doc in its `requires:` and
`depends_on:` before writing code.
