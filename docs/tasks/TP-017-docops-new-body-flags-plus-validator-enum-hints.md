---
title: docops new --body flags plus validator enum hints
status: backlog
priority: p1
assignee: unassigned
requires: [ADR-0020]
depends_on: [TP-008, TP-003]
---

# docops new --body flags plus validator enum hints

## Goal

Let `docops new` create a doc with real content in one CLI call, so
agents (and scripts) never have to write-after-create to replace the
stub. Also: make validator enum errors self-diagnosing by listing the
allowed values inline.

## Acceptance

### docops new --body flags

- `docops new <kind> "title" ... --body -` — read the document body
  from stdin. The body replaces the default stub entirely.
- `docops new <kind> "title" ... --body-file <path>` — read the body
  from a file. Same semantics as `--body -` but sourced from disk.
- Mutually exclusive: passing both `--body -` and `--body-file` exits
  with code 2 and a clear usage error.
- When either flag is set, `$EDITOR` is never launched (implicit
  `--no-open`). `--json` output is unchanged (`{"id": "...", "path":
  "..."}`).
- The body is written verbatim below the frontmatter fence. DocOps
  prepends its own frontmatter; callers must not include a leading
  `---` fence in their body content. If they do, the resulting file
  will still parse (yaml.v3 is tolerant of extra `---` separators)
  but the validator surfaces it cleanly — document the constraint in
  the flag help text rather than sanitizing.
- Flag help text on `--body`:
  `read the document body from stdin; replaces the default stub`.
- Flag help text on `--body-file`:
  `read the document body from <path>; replaces the default stub`.
- Tests in `internal/newdoc/newdoc_test.go` covering:
  - Stdin body round-trip (`Options.BodyReader` or equivalent API).
  - File body round-trip.
  - Mutual exclusion error.
  - That `--no-open` is implicit when body is provided.
- Update `cmd/docops/cmd_new.go` usage string.
- Update `templates/skills/docops/new-task.md`, `new-adr.md`,
  `new-ctx.md` to document and prefer the stdin pattern for agents:
  ```
  docops new task "Title" --requires ADR-0004 --body - <<'EOF'
  ## Goal
  …
  ## Acceptance
  …
  EOF
  ```

### Validator enum-error messages

- Every `invalid-enum` (or equivalent) finding produced by
  `internal/schema/validate.go` (or wherever enum checks live)
  includes the allowed values in the `Message` field. Format:
  `status "proposed" is not one of: draft, accepted, superseded`.
- Fix covers at least: ADR `status`, ADR `coverage`, Task `status`,
  Task `priority`, CTX `type` (if the project has configured
  `context_types`; if empty, no enum, no hint needed).
- Tests that string-match the old message (if any exist in
  `internal/schema/validate_test.go` or `internal/validator/...`) are
  updated to match the new shape.
- `Rule` field values do not change — external consumers use
  `Rule` as the stable contract (per TP-003 design).
- Add one new test asserting the new message format for each of the
  five enum fields above.

### Cross-cutting

- `go test -race ./...` passes clean. `go vet ./...` passes.
- `make build` produces a working binary.
- Smoke test: from the project root, run
  `echo "## Goal\n\nDummy." | ./bin/docops new task "smoke" --requires ADR-0004 --body - --json`,
  verify the resulting file has the piped body, then delete it and
  reset `docs/.docops/counters.json`.

## Notes

Do not embed parser logic in `--body` handling — just read bytes and
write them below the frontmatter. The parser and validator will
surface any malformed structure as a normal validation error on the
next `docops validate`.

The flag name is `--body` (not `--content` or `--text`) for two
reasons: it mirrors how markdown documents are described (frontmatter
+ body), and it pairs cleanly with `--body-file`. Stick with this.

The validator change is a one-liner per finding where the finding is
produced, not a wrapper around the finding at render time. Keep the
structured fields (`Rule`, `Field`, etc.) untouched; only `Message`
grows the hint.
