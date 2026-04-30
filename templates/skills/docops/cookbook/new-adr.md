---
name: new-adr
description: Create a new ADR (Architecture Decision Record) for a design or process decision. Use when the user is about to write code that encodes a decision not yet recorded.
---

# Cookbook: new-adr

## Context
Capture a decision under `docs/decisions/`. Default fields on creation:
`status: draft`, `coverage: required`, `date: <today>`. Status enum:
`draft | accepted | superseded`. `coverage: not-needed` is allowed but
requires a short justification in the body.

**Title the *pattern or decision*, not the first application** — the
ADR should still read well when use #2 or #3 lands. Prefer "Provider
capability registry" over "ZoomInfo as flagged provider". Mention the
triggering case in the Context section, not the title.

## Input
Title (always); optional `--related <CSV of IDs>`; body via `--body -`
heredoc or `--body-file <path>` (mutually exclusive; both imply
`--no-open`).

## Steps
1. Create and populate in one call:

   ```
   docops new adr "Title" --related ADR-xxxx,CTX-yyy --body - <<'EOF'
   ## Context
   What problem forced this decision.

   ## Decision
   What will be done.

   ## Rationale
   Why this option.

   ## Consequences
   What this enables or restricts.
   EOF
   ```

   Or from a file:

   ```
   docops new adr "Title" --body-file /path/to/body.md --json
   ```

2. Pair with `/docops:new-task` so the decision isn't orphaned. At
   least one task should cite the new ADR once accepted.

## Confirm
ADR ID created, default status (`draft`) and coverage, and the paired
task IDs (if any) that will cite it.
