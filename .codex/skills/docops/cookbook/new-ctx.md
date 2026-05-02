---
description: Create a new CTX (Context) document — PRD, memo, research, design brief, constraints, or interview notes. Use when capturing stakeholder input or guardrails.
---

# Cookbook: new-ctx

## Context
Capture the *why* — stakeholder intent, constraints, research — under
`docs/context/`. Each CTX should be focused: one PRD per feature, one
memo per guardrail, one research note per investigation. CTX docs are
the raw material ADRs cite when justifying a decision.

## Input
Title, `--type <kind>`, body. Allowed `--type` values come from
`docops.yaml` → `context_types`. Defaults shipped with init: `prd`,
`design`, `research`, `notes`, `memo`, `spec`, `brief`. Body via
`--body -` heredoc or `--body-file <path>` (mutually exclusive; both
imply `--no-open`).

## Steps
1. Create and populate in one call:

   ```
   docops new ctx "Title" --type memo --body - <<'EOF'
   ## Summary
   Capture the stakeholder input or constraint here.
   EOF
   ```

   Or from a file:

   ```
   docops new ctx "Title" --type brief --body-file /path/to/body.md --json
   ```

## Confirm
CTX ID created, type used, and (if applicable) which ADR or task should
cite it next.
