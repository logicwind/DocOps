---
description: Situational awareness — read STATE.md, run audit, and name the next action in one go. Use at session start or when the user asks "where are we?" / "what's next?".
---

# Cookbook: progress

## Context
Summarise project state and recommend one next action. Briefing should
fit in ≤10 lines. Don't regenerate STATE.md unless the repo changed —
running `docops state` is cheap but creates a commit-worthy diff.

## Input
None.

## Steps
1. Run, in order:

   ```
   docops state
   docops audit
   docops next
   ```

2. (Optional) `docops list --kind TP --status active --json` for the
   active-work line.

## Confirm
A briefing with these lines:
- **Counts:** doc totals + needs-attention from STATE.md.
- **Active work:** tasks currently `task_status: active`.
- **Audit gaps:** one line per finding (skip if 0/0/0).
- **Next task:** what `docops next` picked, with its `requires:` IDs.
- **Recommendation:** one of `/docops:next`, `/docops:new-ctx`,
  `/docops:new-adr`, `/docops:new-task`, `/docops:close <TP-ID>`
  depending on state.
