---
name: progress
description: Situational awareness — read STATE.md, run audit, and name the next action in one go. Use at session start or when the user asks "where are we?" / "what's next?".
---

# /docops:progress

Summarise project state and recommend one next action.

Run these, in order:

```
docops state
docops audit
docops next
```

Then produce a single short briefing:

- **Counts:** doc totals and anything in needs-attention from STATE.md.
- **Active work:** tasks currently `status: active` (from `docops list --kind TP --status active --json`).
- **Audit gaps:** one line per finding; skip if audit is empty.
- **Next task:** whatever `docops next` picked, with its `requires:` IDs.
- **Recommendation:** one of `/docops:next`, `/docops:new-ctx`, `/docops:new-adr`, `/docops:new-task`, or `/docops:close <TP-ID>` depending on state.

Keep the briefing under 10 lines. Do not regenerate STATE.md unless
something in the repo changed — running `docops state` is cheap and
idempotent, but a dirty diff shows up in git.
