---
name: new-task
description: Create a new DocOps task with required ADR/CTX citations. Use when the user says "add a task" or after discovering a gap that needs a follow-up.
---

# /docops:new-task

Create a new task file under `docs/tasks/`.

Every task must cite ≥1 ADR or CTX in `requires:` — the validator enforces this.

Preferred pattern for agents — create and populate in one call:

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

If you already have the body in a file:

```
docops new task "Title" --requires ADR-0004 --body-file /path/to/body.md --json
```

`--body` and `--body-file` are mutually exclusive. Both imply `--no-open` (no editor launch). A leading `---` in the body is treated as body content, not frontmatter.

If the user cannot name a citation, stop and help them find or write one:

- Structural decision? Draft an ADR first via `/docops:new-adr`.
- Stakeholder input? Capture it as CTX via `/docops:new-ctx`.

**Task status enum:** `backlog` | `active` | `blocked` | `done` (literal values; not `in_progress`, `wip`, etc.). Default on creation is `backlog`. To start work, edit the task's `status:` frontmatter to `active` and run `docops refresh`.
