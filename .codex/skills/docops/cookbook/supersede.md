---
description: Replace a published ADR with a new one when the decision itself changes — reversal, different choice, scope flip. Use when the original ADR is no longer the source of truth. Triggers on "supersede ADR-NNNN", "ADR-NNNN is wrong, write a new one", or any decision-change request.
---

# Cookbook: supersede

## Context
Decision changed → write a new ADR that supersedes the old one. Old ADR
remains historical; new ADR is the live source of truth. For typo / dead-link
fixes route to `cookbook/amend.md`; for scope shifts route to
`cookbook/revise.md`.

## Input
- **Old ADR ID**.
- **New decision** — full draft, written from scratch with its own title,
  tags, and coverage. The new title should reflect the new decision (not
  describe the change).

## Steps
1. Read the old ADR:

   ```
   docops get <OLD-ADR-ID>
   ```

   Note `referenced_by` — open tasks may need re-pointing.

2. Draft the new ADR:

   ```
   docops new adr "<new title>" --related <OLD-ADR-ID> --body - <<'EOF'
   ## Context
   <why the old decision no longer fits>

   ## Decision
   <new decision in full>

   ## Rationale
   <why this is the right call now>

   ## Consequences
   <what changes downstream>
   EOF
   ```

3. Edit the new ADR's frontmatter — add `supersedes: [<OLD-ADR-ID>]`. Do
   **not** hand-edit the old ADR's `superseded_by:` (computed in the index).

4. Refresh + validate:

   ```
   docops refresh
   docops validate
   ```

5. Surface the old ADR's open `referenced_by` tasks — ask the user which
   should re-point to the new ADR. Do not silently re-point.

## Confirm
New ADR ID and title, that it supersedes `<OLD-ADR-ID>` (now `superseded`),
the list of `referenced_by` tasks awaiting the user's call, and that
`refresh`/`validate` succeeded.
