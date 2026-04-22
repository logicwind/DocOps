---
name: state
description: Read the current project state from docs/STATE.md and summarise counts, needs-attention, and active work for the agent. Use when a session opens or when you need a quick read on where things stand.
---

# /docops:state

Show the current DocOps project snapshot.

Run:

```
docops state
cat docs/STATE.md
```

Summarise the output in 5 bullets or fewer:

- Doc counts.
- "Needs attention" (if any).
- Active tasks (if any).
- Most recent commits touching docs.
- One-line recommendation for the next action (usually `/docops:next` or `/docops:audit`).

Do not regenerate STATE.md unless the user asks — `docops state` is cheap, and regeneration creates a commit-worthy diff.
