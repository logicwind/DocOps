---
name: state
description: Read the current project state from docs/STATE.md and summarise counts, needs-attention, and active work. Use when a session opens or when you need a quick read on where things stand.
---

# Cookbook: state

## Context
Snapshot the project's current shape from `docs/STATE.md`. Cheap and
idempotent. Don't regenerate unless the user asks — regeneration creates
a commit-worthy diff.

## Input
None.

## Steps
1. Run:

   ```
   docops state
   cat docs/STATE.md
   ```

## Confirm
≤5 bullets: doc counts; "needs attention" if any; active tasks if any;
recent doc-touching commits; one-line recommendation (usually
`/docops:next` or `/docops:audit`).
