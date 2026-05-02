---
description: Given a CTX (PRD, memo, research note), draft one ADR and one or more tasks that cite it. Human-confirmed before write. Use when turning stakeholder input into actionable work.
---

# Cookbook: plan

## Context
Convert a CTX into an ADR + tasks. Always human-confirmed before any
write — show proposed frontmatter and body summary, then ask. Keep the
ADR `status: draft` unless the user explicitly accepts it.

## Input
A CTX ID. Ask once if missing.

## Steps
1. Read the CTX:

   ```
   docops get <CTX-ID>
   ```

2. Draft **one ADR** capturing the decision the CTX implies. Propose
   title, Context / Decision / Rationale / Consequences body. Confirm
   before writing.

3. Draft **one or more tasks** that cite the ADR. Each carries
   priority, an acceptance checklist, and (if relevant) `depends_on`.
   Confirm the full set before writing.

4. Write via heredocs so drafts land populated on creation:

   ```
   docops new adr "Title" --related <CTX-ID> --body - <<'EOF'
   ## Context
   ...
   ## Decision
   ...
   ## Rationale
   ...
   ## Consequences
   ...
   EOF

   docops new task "Title" --requires <ADR-ID> --priority p1 --body - <<'EOF'
   ## Goal
   ...
   ## Acceptance
   - ...
   EOF
   ```

5. Refresh:

   ```
   docops refresh
   ```

## Confirm
ADR ID + draft status, the TP IDs that cite it, the priority/assignee
assignments, and that `refresh` returned OK.
