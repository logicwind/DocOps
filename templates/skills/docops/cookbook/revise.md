---
description: Tighten or expand the scope of a published ADR without flipping the decision. Use when the call still stands but its boundaries have shifted. Triggers on "narrow ADR-NNNN's scope", "ADR-NNNN should also cover X", or scope-shift requests heavier than amend but lighter than supersede.
---

# Cookbook: revise

## Context
Third lane between amend (text fix, no decision change) and supersede
(decision changed). Revise is for the case where the decision still
stands but its boundaries have moved — usually a narrowing or widening.
Mechanically: a `clarification`-kind amendment with `--section` flags,
plus a follow-up task if the shift is load-bearing.

If user describes a typo / dead link → `cookbook/amend.md`. If user
describes a different decision → `cookbook/supersede.md`.

## Input
- **ADR ID**.
- **Direction** — narrowing or expansion.
- **Affected sections** — usually `## Decision`, `## Out of scope`, or
  `## Consequences`.
- **Why now** — anchor: a CTX, an incident, or a follow-up TP.

## Steps
1. Verify the ADR (`docops get <ADR-ID>`); confirm `accepted`. Note
   `referenced_by` — narrowings may invalidate cited tasks.
2. Append the clarification:

   ```
   docops amend <ADR-ID> \
     --kind clarification \
     --summary "<one-line scope shift>" \
     --section "<affected heading>" \
     --ref <CTX-NNN | TP-NNN | ADR-NNNN that motivated this>
   ```

   Add `--marker-at "<exact substring>"` if the shift attaches to a
   specific sentence.
3. If the shift implies code or doc work, open a follow-up task:

   ```
   docops new task "<title>" --requires <ADR-ID> --body - <<'EOF'
   ## Goal
   <what to deliver>

   ## Acceptance
   <how we know it's done>
   EOF
   ```

4. Refresh + validate:

   ```
   docops refresh
   docops validate
   ```

5. Surface affected `referenced_by` tasks for user review.

## Confirm
ADR ID with the clarification appended, affected sections recorded, the
follow-up TP ID if opened, and the list of `referenced_by` tasks the
user should review against the new scope.
