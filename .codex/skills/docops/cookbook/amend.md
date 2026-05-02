---
description: Append an amendment to a published ADR — for editorial fixes, errata, dead-link repair, late-binding facts, or clarifications. Use when an ADR's decision is unchanged but its text needs touch-up. Triggers on "amend ADR-NNNN", "fix typo in ADR-NNNN", "ADR-NNNN's link is broken".
---

# Cookbook: amend

## Context
ADRs are append-only. Amend appends a structured entry to frontmatter +
an `## Amendments` subsection; the decision body is never rewritten.
If the *decision* itself changes, route to `cookbook/supersede.md`.
If only its scope shifts, route to `cookbook/revise.md`.

## Input
- **ADR ID** (`ADR-NNNN`).
- **Kind** — `editorial` (typo / wording) | `errata` (factually wrong) |
  `clarification` (adds framing, no decision change) | `late-binding`
  (fills a former placeholder).
- **Summary** — one short line.
- Optional `--ref <TP-NNN | ADR-NNNN | PR | issue>`, `--by <handle>`,
  `--marker-at "<exact substring>"` (must be unique in body),
  `--section "<heading>"` (repeatable).

## Steps
1. Confirm this is the amend lane (decision unchanged). Else route out.
2. Verify:

   ```
   docops get <ADR-ID>
   ```

   Should be `accepted`. Flag if `draft` or already `superseded`.
3. Run:

   ```
   docops amend <ADR-ID> --kind <kind> --summary "<one-line>"
   ```

   Add `--ref` / `--by` / `--marker-at` / `--section` as available.
4. Refresh:

   ```
   docops refresh
   ```

   Then `docops validate` — must be 0 errors.

## Confirm
ADR ID, kind, summary appended, whether the inline `[AMENDED ...]` marker
was inserted (CLI prints this), the new amendment count, and that
`refresh`/`validate` succeeded.
