---
description: Append an amendment to a published ADR — for editorial fixes, errata, dead-link repair, late-binding facts, or clarifications. Use when an ADR's decision is unchanged but its text needs touch-up. Triggers on "amend ADR-NNNN", "fix typo in ADR-NNNN", "ADR-NNNN's link is broken", or any ADR change request that is not a new decision.
---

# Cookbook: amend an ADR

## Context

ADRs are append-only. When a published ADR has a typo, a dead link, a
late-binding fact (e.g. a name that was a placeholder at decision time),
or a reader-prompted clarification, do **not** silently edit the body
and do **not** write a superseding ADR. Use `docops amend` (per
ADR-0025) — it appends a structured amendment to frontmatter and to
an `## Amendments` subsection at the bottom of the ADR. The decision
body itself is never rewritten.

If the *decision* changes, this is the wrong chapter — use
[supersede.md](supersede.md). If the decision stands but its scope
shifts, use [revise.md](revise.md).

## Input

The user identifies an ADR and the change. From their phrasing,
extract:

- **ADR ID** (`ADR-NNNN`).
- **Kind** — one of:
  - `editorial` — typo, wording polish, formatting
  - `errata` — factually wrong statement or stale text
  - `clarification` — adds framing/explanation; doesn't change the call
  - `late-binding` — fills in a placeholder/unknown that's now known
- **Summary** — one short line capturing the change.
- **Optional `--ref`** — task or follow-up ADR/PR that motivated the
  amendment (e.g. `--ref TP-024`).
- **Optional `--by`** — author handle. Defaults to `$DOCOPS_USER`,
  then git `user.name`, then `$USER`.
- **Optional `--marker-at "<substring>"`** — exact substring in the
  ADR body where an inline `[AMENDED YYYY-MM-DD kind]` marker should
  be inserted. The substring must be unique.
- **Optional `--section <heading>`** — repeatable; `affects_sections`
  hint indexed for the SPA viewer.

If kind is ambiguous from the user's phrasing, pick the closest from
the rubric above — do not ask unless the choice is genuinely between
amend and supersede/revise (those are different chapters).

## Steps

### 1. Confirm this is the amend lane

Re-read the user's request. If they describe a *change of decision*,
stop and route to `cookbook/supersede.md`. If they describe a *scope
shift without a decision change*, route to `cookbook/revise.md`. Only
proceed here for editorial/errata/clarification/late-binding.

### 2. Verify the ADR exists

```
<DOCOPS_BIN> get <ADR-ID>
```

Confirm the ADR is `accepted` (not `draft`, not already `superseded`).
Amending a `draft` ADR is allowed but unusual — flag it to the user.

### 3. Run the amendment

Minimum form:

```
<DOCOPS_BIN> amend <ADR-ID> --kind <kind> --summary "<one-line>"
```

With ref + author + inline marker:

```
<DOCOPS_BIN> amend <ADR-ID> \
  --kind <kind> \
  --summary "<one-line>" \
  --ref <TP-NNN | ADR-NNNN | PR url | issue ref> \
  --by "<author>" \
  --marker-at "<exact substring already in the ADR>"
```

If the user's intent maps cleanly to a section heading, also pass
`--section "<heading>"` (repeatable) so the viewer can highlight the
affected scope.

### 4. Refresh the index

```
<DOCOPS_BIN> refresh
```

The amendment is already on disk; this rebuilds `docs/.index.json`
and `docs/STATE.md` so reverse edges and `recent_amendments` reflect
the change.

### 5. Validate

```
<DOCOPS_BIN> validate
```

Should report 0 errors. If validation fails, the amend produced an
invalid frontmatter shape — surface the error to the user; do not
hand-fix.

## Confirm

Report back:

- The ADR ID, kind, and one-line summary that was appended.
- Whether an inline `[AMENDED ...]` marker was inserted (CLI prints
  this — pass it through).
- The new amendment count on the ADR (visible in `docops get <ADR-ID>`
  output).
- That `docops refresh` succeeded and the SPA viewer / STATE.md will
  surface the amendment.

Do **not** offer to also commit the change unless the user asked —
many users batch amend + commit themselves.
