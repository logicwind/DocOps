---
title: Implement amendments as first-class decision metadata
status: done
priority: p2
assignee: claude
requires: [ADR-0025]
depends_on: []
---

## Goal

Ship the machine layer behind ADR-0025: the `amendments:` frontmatter
field, the `docops amend` CLI, validator + audit + index + state
integration, and the JSON Schema extension.

End state: `docops amend ADR-0019 --kind editorial --summary "..."
--marker-at "<substring>" --body-file -` runs end-to-end, commits
atomically, and `docops validate` accepts the result.

## Acceptance

### Schema

- `internal/schema` (ADR struct) gains `Amendments []Amendment` with
  YAML tag `amendments,omitempty`.
- `Amendment` struct:
  - `Date` (string, ISO; required)
  - `Kind` (enum: `editorial | errata | clarification | late-binding`; required)
  - `By` (string; required)
  - `Summary` (string; required, non-empty)
  - `AffectsSections` ([]string; optional)
  - `Ref` (string; optional)
- `docs/.docops/schema/decision.schema.json` updated — vscode-yaml
  surfaces the field and lints `kind` inline.

### `docops amend`

- Flag surface matches ADR-0025 §Decision/CLI exactly:
  `-kind`, `-summary`, `-section` (repeatable), `-ref`, `-by`,
  `-body`, `-body-file`, `-marker-at`, `-no-open`, `-json`.
- `-body` accepts literal text or `-` (stdin); mirrors `docops new`.
- `-marker-at "<substring>"` inserts the `[AMENDED YYYY-MM-DD kind]`
  marker after the first exact-match occurrence. If the substring
  is missing or not unique, fail with a clear error that lists
  candidate positions.
- When no `--body*` is passed, a three-line default stub is written.
- The new `### YYYY-MM-DD — <summary> (<kind>)` subsection is
  appended to an `## Amendments` section (created if absent) placed
  before any trailing sections.
- File write is atomic (tmp-file + rename). No partial writes.
- `--json` emits `{ "adr": "ADR-NNNN", "amendment_index": N, "path": "..." }`.
- Exit 0 on success; 2 on invalid flags; 1 on semantic failure.

### `docops validate`

- Rejects entries with missing required fields or unknown `kind`.
- Enforces inline-marker ↔ frontmatter-entry correspondence:
  - Every `[AMENDED YYYY-MM-DD kind]` in body matches a frontmatter
    entry with same date + kind.
  - Every frontmatter entry has either a matching inline marker or
    a non-empty `affects_sections`.
- Amendments on `superseded` or `rejected` ADRs emit a warning, not
  an error.

### `docops audit`

- `amendment_count >= 5` on a single ADR → flag "consider supersede".
  Threshold configurable in `docops.yaml` under `gaps:`.
- ADRs whose body was git-edited in a commit that did not also touch
  the `amendments:` frontmatter → flag "hand-edit drift".
- Amendments whose `ref` is a task id and the task is still `backlog`
  after `gaps.amendment_stale_days` → flag "amendment stalled".

### `docops index` + `docops state`

- `docs/.index.json` gains `amendments` on every decision record +
  a top-level `recent_amendments` list windowed by
  `recent_activity_window_days`.
- `STATE.md` gains a "Recent amendments" section rendering the same
  list.

### Tests

- Unit tests for `docops amend` flag parsing, marker insertion,
  atomic write, `--json` output.
- Integration test: run `docops new adr` + `docops amend` + `docops
  validate` end-to-end; assert clean.
- Negative tests: missing kind, duplicate inline marker, body-file
  unreadable, marker-at substring absent / ambiguous.
- Regression test: existing fixtures without `amendments:` still
  validate (additive field; absence is fine).

## Notes

- The marker-insertion algorithm is the subtle part. Use runes, not
  bytes; be careful with grapheme clusters in summaries used for
  subsection titles. Consider truncating subsection titles at 60
  chars to keep TOCs readable.
- `by:` default: `$DOCOPS_USER` if set, else `git config user.name`,
  else `$USER`, else fail.
- Reserve `kind: supersede-hint` for a future ADR if users want
  "amendment that signals the decision should be reconsidered" —
  out of scope for this task.
- Consider emitting a follow-up message at the end of `docops amend`:
  "tip: run `docops refresh` to rebuild .index.json + STATE.md."

## Out of scope

- Backfill of existing ADRs (tracked in separate task).
- `docops supersede` CLI (separate follow-up; nominally parallel in
  shape to `docops amend`).
- UI for rendering amendment timelines — CLI only for this task.
