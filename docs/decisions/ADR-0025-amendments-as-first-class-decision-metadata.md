---
title: Amendments as first-class decision metadata
status: draft
coverage: required
date: "2026-04-23"
supersedes: []
related: [ADR-0019, ADR-0018]
tags: []
---

## Context

ADRs are immutable by convention. This is the right default — decision
history must not be silently rewritten. But the convention creates a
dead zone for the most common kind of post-acceptance change:
**editorial correction that does not alter the decision**.

The live case driving this ADR (2026-04-23):

- `ADR-0019` named the follow-up Homebrew tap repo
  `logicwind/homebrew-docops` and the Scoop bucket repo
  `logicwind/scoop-docops`. When the tap stand-up task (`TP-024`)
  reached planning, the org-wide convention — `logicwind/homebrew-tap`
  + `logicwind/scoop-bucket`, matching Vercel/HashiCorp/Fly.io — was
  preferred. The decision (defer tap publish to a follow-up release)
  was unchanged. Only two names flipped.
- Following docops convention, we attempted to add an `amendments:`
  array to ADR-0019's frontmatter. `docops validate` rejected it:
  the ADR schema has no such field. The change had to be placed in an
  HTML comment instead — losing machine-readability.

This is the first-class amendments gap. Other real-world scenarios
that hit the same wall:

- **Errata.** "We wrote X; we meant Y." No decision change.
- **Dead-link repair.** External references move or 404.
- **Rename pass-through.** Product or repo renames after the ADR
  references the old name.
- **Late-binding facts.** At decision time, a placeholder (e.g. "the
  future auth provider") was used; later the concrete name is known.
- **Clarifications prompted by reader feedback.** New framing of the
  same decision, not a new decision.

None of these justify a superseding ADR (heavyweight; creates ADR
churn; makes "what's current?" harder to answer). None of them should
be silent edits (violates immutability; lost audit trail). The
missing middle is **amendments**.

Prior art:

- **IETF RFCs** ship **errata** as a first-class separate object with
  its own status workflow.
- **Python PEPs** permit minor corrections inline with a changelog
  footer; substantive changes require a new PEP.
- **Legislation** uses explicit "as amended on YYYY-MM-DD" stamps and
  cross-references the amending statute.
- The ADR canon (Nygard; MADR) is silent on amendments — an unfilled
  niche DocOps can occupy.

## Decision

Amendments are a **first-class additive axis** on ADRs, orthogonal to
the existing `status` (decision state) and `implementation` (code
state) axes. An ADR can simultaneously be `accepted`, `done`, and
carry N amendments.

### Frontmatter schema

```yaml
amendments:
  - date: 2026-04-23                 # ISO date (required)
    kind: editorial                  # enum (required)
    by: nix                          # author / agent handle (required)
    summary: "Tap/bucket renamed to org-wide convention"
    affects_sections: ["v0.1.0 scope"]   # optional — free-form section hints
    ref: null                        # optional — ADR id, PR URL, issue ref, or task id
```

`kind` enum:

| Value | Meaning |
|---|---|
| `editorial` | Typo, dead link, rename pass-through |
| `errata` | We wrote X; we meant Y — factual correction |
| `clarification` | Same decision, new framing / added examples |
| `late-binding` | Placeholder at decision time, now concrete |

`editorial` and `late-binding` are non-semantic. `errata` and
`clarification` are semantic-adjacent but must not re-decide anything.

### Hard rule

**Amendments must not change what the ADR decided.** If the change
would invert, narrow, re-scope, or add conditions to the decision,
the correct move is to write a superseding ADR. Tooling cannot
enforce this perfectly (semantic), but `docops validate` enforces
mechanical correlates (see Validation below).

### Body marker

Inline markers sit next to the amended prose:

```markdown
The tap will live at `logicwind/homebrew-docops` [AMENDED 2026-04-23 editorial].
```

Every inline marker must reference a frontmatter `amendments:` entry
with the same date + kind. Every frontmatter entry should have at
least one matching marker (unless `affects_sections` is set for a
broad structural amendment that does not point to a single line).

### Amendments section

ADRs with amendments gain an `## Amendments` section near the end
(before any appendices), with one subsection per amendment:

```markdown
## Amendments

### 2026-04-23 — <short title> (editorial)

Summary, what-is-unchanged, what-changed, by.
```

The rendered view in `docs/.index.json` and `STATE.md` links the
frontmatter entry to the body subsection.

### CLI

`docops amend` mirrors the flag surface of `docops new` exactly so
agents can script amendments end-to-end without invoking `$EDITOR`:

```
usage: docops amend <ADR-ID> [flags]
  -kind string           editorial | errata | clarification | late-binding  (required)
  -summary string        one-line human summary (required)
  -section string        repeatable; affects_sections entry
  -ref string            optional follow-up ADR id, PR URL, issue ref, or task id
  -by string             author handle (defaults to $USER or git user.name)
  -body string           amendment body — either literal text or `-` for stdin
  -body-file string      amendment body read from <path>
  -marker-at string      optional literal string in the ADR body to prepend the [AMENDED …] marker to; exact substring match required
  -no-open               skip opening $EDITOR after the write
  -json                  emit {adr, amendment_index, path} instead of human output
```

When invoked:

1. Resolve ADR file by ID. Fail fast if missing.
2. Parse frontmatter; validate `kind` against enum.
3. Append a new entry to `amendments:` with all passed flags.
4. If `--marker-at "<exact substring>"` is given, locate the
   substring in the body and insert the `[AMENDED YYYY-MM-DD kind]`
   marker immediately after it. Exact-match only — if the substring
   is not unique or not present, fail with a clear error listing
   candidates.
5. Append an `### YYYY-MM-DD — <summary-first-N> (<kind>)` subsection
   to the `## Amendments` section (creating the section if absent).
   Body content comes from `--body` / `--body-file` / stdin;
   otherwise a three-line default stub is inserted.
6. Write file atomically. Exit non-zero if validation would fail.
7. Unless `--no-open`, open `$EDITOR` positioned at the new
   subsection so the human can expand.

This shape lets a coding agent do:

```sh
docops amend ADR-0019 \
  --kind editorial \
  --summary "Tap/bucket naming: per-tool → org-wide" \
  --section "v0.1.0 scope" \
  --marker-at "logicwind/homebrew-docops" \
  --body-file -                   <<'EOF'
The repos were renamed to logicwind/homebrew-tap and
logicwind/scoop-bucket when the stand-up plan was written
(see TP-024). The deferral decision is unchanged.
EOF
```

End-to-end, non-interactive, no editor, commit-ready.

### Validation rules (`docops validate`)

- `amendments[].kind` is one of the four enum values.
- `amendments[].date` parses as ISO.
- `amendments[].summary` is non-empty.
- Every `[AMENDED YYYY-MM-DD kind]` marker in the body has a
  matching frontmatter entry (same date + kind).
- Every frontmatter entry has either a matching inline marker or a
  non-empty `affects_sections`.
- Amendments on a `superseded` or `rejected` ADR emit a warning, not
  an error (editorial fixes on archived decisions are allowed).

### Audit rules (`docops audit`)

- Flag ADRs with ≥ 5 amendments — suggests the decision itself may
  be drifting and a superseding ADR would be cleaner.
- Flag ADRs where the body has been git-edited without a
  corresponding frontmatter amendment entry in the same commit
  (hand-edit drift; amendments should go through `docops amend`).
- Flag amendments older than `gaps.amendment_stale_days`
  (configurable in `docops.yaml`) whose `ref` is a task id and the
  task is still `backlog` — amendment is "we said we'd do X; X still
  isn't done."

### Index + STATE.md

- `docs/.index.json` gains `amendments` on each decision record +
  top-level `recent_amendments` list (last N, windowed by
  `recent_activity_window_days`).
- `STATE.md` gains a "Recent amendments" section listing amendments
  within the activity window.

### JSON Schema

`docs/.docops/schema/decision.schema.json` is extended so the
`redhat.vscode-yaml` extension autocompletes `amendments:` entries
and lints the `kind` enum inline in the editor.

## Consequences

- Editorial and errata-style corrections on accepted ADRs get a
  durable, machine-readable log — no more HTML-comment workarounds
  or silent edits.
- The supersede-vs-amend decision is now a deliberate choice.
  `docops audit` surfaces when amendments are piling up and
  supersession would be cleaner.
- Non-interactive tooling (agents, CI scripts) can amend without an
  editor — critical for the LLM-agentic development story.
- ADR immutability is preserved in spirit: the decision body is not
  rewritten; amendments are additive-only appends with their own
  section.
- JSON Schema grows. Editors surface the new field immediately.
- A small ecosystem question: existing Nygard/MADR ADRs from other
  projects won't have `amendments`, so importing ADRs from other
  repos keeps working (additive field; absence is fine).
- The ADR for a product rename (e.g. future "docops" → something)
  becomes an amendment pass over every affected ADR, not a rewrite.

## Rollout

1. This ADR lands `accepted / not-started`.
2. Task `TP-NNN — Implement amendments first-class` — add schema
   field, extend `docops validate` + `audit` + `index` + `state`,
   ship `docops amend`, update `decision.schema.json`.
3. Task `TP-NNN+1 — Backfill ADR-0019 amendment` — the case that
   motivated this ADR. Convert the HTML-comment amendment block on
   ADR-0019 to a proper frontmatter entry using `docops amend`.
4. Documentation: extend the "ADR lifecycle" section of the docops
   user docs with the supersede-vs-amend decision tree.

The immutability rule remains: the original decision body is never
rewritten. Amendments are additive appends and extra frontmatter.
