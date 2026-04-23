---
name: plan
description: Given a CTX (PRD, memo, research note), draft one ADR and one or more tasks that cite it. Human-confirmed before write. Use when turning stakeholder input into actionable work.
---

# /docops:plan

Convert context into a decision plus tasks.

Ask for the CTX ID if not provided. Read it, then draft:

1. **One ADR** capturing the decision the CTX implies or demands. Propose
   title, Context/Decision/Rationale/Consequences body. Confirm with the
   user before writing.

2. **One or more tasks** that cite the ADR. Each task should carry
   priority, an acceptance checklist, and (if relevant) a depends-on
   reference to other tasks. Confirm the full set before writing.

Write via `--body -` heredocs so drafts land populated on creation (no
stub-then-rewrite round-trip):

```
docops new adr "Title" --related ADR-xxxx --body - <<'EOF'
## Context
...
## Decision
...
## Rationale
...
## Consequences
...
EOF

docops new task "Title" --requires ADR-0026 --priority p1 --body - <<'EOF'
## Goal
...
## Acceptance
- ...
EOF
```

After all writes, run `docops refresh` to regenerate index and STATE.md.

Rules:

- Every task must cite ≥1 ADR or CTX in `requires:`. If the user cannot
  name a citation, stop and draft the missing ADR/CTX first.
- Keep ADR `status: draft` unless the user explicitly accepts it.
- Don't write anything the user hasn't confirmed. Show proposed
  frontmatter and body summary, then ask.
