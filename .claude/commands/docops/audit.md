---
name: audit
description: Run docops audit to surface structural coverage gaps (stale ADRs, tasks with no ADR, CTX without derived links) and offer to draft fixer tasks. Use when the user asks "what's broken" or before a release checkpoint.
---

# /docops:audit

Run the structural audit and help the user close gaps.

```
docops audit
```

Group findings by rule, not by doc. For each rule, offer one of:

- Draft a new ADR (if a decision is missing) via `/docops:new-adr`.
- Draft a new task with citations (if a decision has no follow-up) via `/docops:new-task`.
- Mark an ADR `coverage: not-needed` (if the decision does not require implementation — only after asking the user).

Never edit frontmatter silently. Always show the change and confirm.
