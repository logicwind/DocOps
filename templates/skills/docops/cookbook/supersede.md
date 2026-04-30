---
description: Replace a published ADR with a new one when the decision itself changes — reversal, different choice, scope flip. Use when the original ADR is no longer the source of truth. Triggers on "supersede ADR-NNNN", "ADR-NNNN is wrong, write a new one", or any decision-change request.
---

# Cookbook: supersede an ADR

## Context

When the *decision* recorded in a published ADR has changed — a
reversal, a different choice, a scope flip — write a new ADR that
**supersedes** the old one. The old ADR remains as historical record;
the new ADR is the live source of truth. This keeps the audit trail
intact without rewriting prior decisions.

Use this chapter only when the call itself changes. For typo / dead
link / late-binding fixes (decision unchanged), use
[amend.md](amend.md). For scope tightening or expansion that doesn't
flip the decision, use [revise.md](revise.md).

## Input

The user identifies the old ADR and describes the new decision. From
their phrasing, extract:

- **Old ADR ID** (`ADR-NNNN`) — the one being replaced.
- **New decision** — a full draft, not just the diff. The new ADR is
  written from scratch; it cites the old one in `supersedes:` so the
  link is computed in the index.
- **Title** for the new ADR — should reflect the new decision, not
  describe the change ("Use SQLite for embedded sessions" not
  "Replace ADR-0014").
- **Tags / coverage** — the new ADR gets its own metadata, not
  inherited.

## Steps

### 1. Confirm this is the supersede lane

If only the *text* of the old ADR is wrong and the call still stands,
route to `cookbook/amend.md`. If the call stands but its scope is
shifting, route to `cookbook/revise.md`. Only proceed here when the
*decision itself* is changing.

### 2. Read the old ADR

```
<DOCOPS_BIN> get <OLD-ADR-ID>
```

Note its title, tags, and any open tasks that cite it
(`referenced_by`). After supersession those tasks may need to be
re-pointed at the new ADR — flag this to the user, do not silently
re-point.

### 3. Draft the new ADR

```
<DOCOPS_BIN> new adr "<new title>" --related <OLD-ADR-ID> --body - <<'EOF'
## Context

<why the old decision no longer fits>

## Decision

<the new decision in full>

## Rationale

<why this is the right call now; cite incidents/data/changes that
forced the rethink>

## Consequences

<what changes downstream — code, docs, processes>

## Out of scope

<adjacencies that this ADR does not address>
EOF
```

`--related` records the linkage. The `supersedes:` field is set in
the next step — the new ADR is created in `accepted` status by
default; if you prefer to land it as `draft` first, add the
`status: draft` line manually after creation.

### 4. Mark the new ADR as superseding the old one

Edit the new ADR's frontmatter. Add (or update) the `supersedes:`
list to include the old ID:

```yaml
supersedes: [<OLD-ADR-ID>]
```

The matching `superseded_by:` field on the old ADR is computed in the
index — do **not** hand-edit the old ADR's frontmatter. The validator
rejects manual edits to reverse-edge fields.

### 5. Refresh and validate

```
<DOCOPS_BIN> refresh
<DOCOPS_BIN> validate
```

Both must succeed. `refresh` regenerates `docs/.index.json` and
`docs/STATE.md`; the index will show the old ADR as `superseded` and
the new one with the back-pointer.

### 6. Re-point active citations (if any)

If the old ADR had `referenced_by` tasks that are still `active` or
`backlog`, the user usually wants those tasks updated to cite the new
ADR. List them:

```
<DOCOPS_BIN> get <OLD-ADR-ID>
```

Do **not** silently re-point. Surface the list and ask the user which
tasks should move to the new ADR.

## Confirm

Report back:

- The new ADR ID, title, and that it supersedes `<OLD-ADR-ID>`.
- The old ADR's status is now `superseded` (computed; visible after
  `refresh`).
- Any `referenced_by` tasks on the old ADR that the user should
  decide about (re-point vs leave historical).
- That `docops refresh` and `docops validate` both succeeded.
