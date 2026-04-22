---
name: next
description: Pick the next actionable DocOps task — respects depends_on, priority, and assignee. Use when the user asks "what should I work on?"
---

# /docops:next

Show the next task ready to work on.

```
docops next
```

Before starting work, load every doc the task cites in `requires:` and `depends_on:` — those are the inputs the user wants you to honour. Do not begin editing code until you have read them.

If no task is actionable, say so and suggest `/docops:audit` to find gaps.
