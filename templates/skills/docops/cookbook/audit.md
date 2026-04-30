---
description: Run docops audit to surface structural coverage gaps — stale ADRs, tasks with no ADR, CTX without derived links — and offer to draft fixer tasks. Use when the user asks "what's broken" or before a release checkpoint.
---

# Cookbook: audit

## Context
Surface structural gaps. Audit is a recommendation surface, not a mutator —
never auto-create tasks or silently edit frontmatter.

## Input
Usually none. Optional `--only <rule>` to scope, `--include-not-needed` to
include opted-out ADRs.

## Steps
1. Run:

   ```
   docops audit
   ```

2. Group findings by rule (the CLI already does this). For each rule, offer **one** closer:
   - **Decision missing** → hand off to `cookbook/new-adr.md`.
   - **Decision without follow-up** → hand off to `cookbook/new-task.md`.
   - **ADR doesn't need impl** → flip `coverage: not-needed` (only after explicit OK; show the diff).
3. Defer anything the user says "leave it" to.

## Confirm
Counts (errors / warnings / info), which rules the user closed vs deferred,
any frontmatter mutations applied. If 0/0/0, say so — done.
