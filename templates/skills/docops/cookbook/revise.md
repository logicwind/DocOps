---
description: Tighten or expand the scope of a published ADR without flipping the decision. Use when the call still stands but its boundaries have shifted. Triggers on "narrow ADR-NNNN's scope", "ADR-NNNN should also cover X", or scope-shift requests that are heavier than amend but lighter than supersede.
---

# Cookbook: revise an ADR

## Context

The third lane between **amend** (decision unchanged, text fix) and
**supersede** (decision changed, new ADR replaces old). A *revision*
is for the case where the decision itself stands, but the scope it
applies to has shifted — usually a narrowing ("this no longer covers
edge case X") or a widening ("this now also applies to Y").

Concretely: a revision is a **clarification-kind amendment** with
`affects_sections:` capturing the scope shift, plus a follow-up task
or ADR that records the new boundary if it's load-bearing.

If the user is asking for a typo or dead-link fix, use
[amend.md](amend.md). If the user is asking for a different decision,
use [supersede.md](supersede.md). Only proceed here when the call
stands but its boundaries are moving.

## Input

The user identifies an ADR and describes the scope shift. Extract:

- **ADR ID** (`ADR-NNNN`).
- **Direction** — narrowing or expansion.
- **Affected sections** — which `## ...` headings in the ADR are
  scope-bearing for the change (typically `## Decision`, `## Out of
  scope`, or `## Consequences`).
- **Why now** — the user should be able to point at a CTX, an
  incident, or a follow-up TP that motivated the scope shift. If
  there's no anchor, this is probably a fresh ADR (use
  [new-adr.md](new-adr.md)) or a supersession in disguise.

## Steps

### 1. Confirm this is the revise lane

Mentally apply the rubric:

- The decision still says "do X" → revise lane.
- The decision now says "do Y instead of X" → supersede lane.
- The decision is fine, only the words are wrong → amend lane.

Only proceed here for case 1 with a real scope shift.

### 2. Verify the ADR

```
<DOCOPS_BIN> get <ADR-ID>
```

Confirm `accepted` status. Note `referenced_by` — narrowing scope
may invalidate citations from tasks that depended on the broader
form.

### 3. Append a clarification amendment

Use `docops amend` with `kind: clarification` and `--section` flags
naming the scope-bearing headings:

```
<DOCOPS_BIN> amend <ADR-ID> \
  --kind clarification \
  --summary "<one-line scope shift>" \
  --section "Decision" \
  --section "Out of scope" \
  --ref <CTX-NNN | TP-NNN | ADR-NNNN that motivated this>
```

Pass `--marker-at "<exact substring>"` if the scope shift attaches
to a specific sentence in the body. Otherwise omit it — the
amendment lives in frontmatter + `## Amendments` subsection.

### 4. If the scope shift is load-bearing, open a follow-up task

A revision often implies code or doc work to align the system with
the new boundary. Open one task per concrete deliverable:

```
<DOCOPS_BIN> new task "<title>" --requires <ADR-ID> --body - <<'EOF'
## Goal
<what to deliver>

## Acceptance
<how we know it's done>

## Notes
Follow-up to the <date> revision of <ADR-ID> — see the clarification
amendment for the scope shift.
EOF
```

If the revision is purely textual (clarifying intent without code
implications), skip this step — the amendment alone is sufficient.

### 5. Refresh and validate

```
<DOCOPS_BIN> refresh
<DOCOPS_BIN> validate
```

Both must succeed.

### 6. Surface affected citations

If `referenced_by` tasks were tied to the *old* scope and the
revision narrows it, those tasks may need to be re-scoped, closed,
or split. List them and surface to the user — do not silently edit
task frontmatter.

## Confirm

Report back:

- The ADR ID, that it received a `clarification` amendment, and the
  one-line scope shift.
- Affected sections recorded (so the SPA viewer can highlight them).
- Any follow-up TP that was opened, with its ID.
- Any `referenced_by` tasks that the user should review against the
  new scope.
