---
name: new-task
description: Create a new DocOps task with required ADR/CTX citations. Use when the user says "add a task" or after discovering a gap that needs a follow-up.
---

# Cookbook: new-task

## Context
Create a task under `docs/tasks/`. Every task must cite ≥1 ADR or CTX in
`requires:` — the validator enforces this. Status enum on creation is
`backlog`; flip to `active` (and `docops refresh`) to start work.

If the user can't name a citation, **stop** and route out:
- Structural decision missing? `cookbook/new-adr.md` first.
- Stakeholder input missing? `cookbook/new-ctx.md` first.

## Input
Title, `--requires <CSV of IDs>`, body. Optional `--priority`,
`--assignee`. Body via `--body -` heredoc or `--body-file <path>`
(mutually exclusive; both imply `--no-open`). A leading `---` in the
body is treated as content, not frontmatter.

## Steps
1. Create and populate in one call:

   ```
   docops new task "Title" --requires ADR-0004 --body - <<'EOF'
   ## Goal
   Describe the goal here.

   ## Acceptance
   - Acceptance criteria.

   ## Notes
   Optional notes.
   EOF
   ```

   Or from a file:

   ```
   docops new task "Title" --requires ADR-0004 --body-file /path/to/body.md --json
   ```

## Confirm
TP ID created, citations recorded, status (`backlog`), and what to do
next (start work or hand off to assignee).
