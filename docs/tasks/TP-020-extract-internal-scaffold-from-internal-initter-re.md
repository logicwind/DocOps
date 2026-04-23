---
title: Extract internal/scaffold/ from internal/initter/ (refactor)
status: done
priority: p2
assignee: unassigned
requires: [ADR-0021, ADR-0023]
depends_on: []
---

## Goal

Lift the helpers that `docops upgrade` will need to share with
`docops init` out of `internal/initter/` and into a new
`internal/scaffold/` package, with **zero behavior change**. Sets up
TP-018 to land cleanly without copy-pasting code.

## Acceptance

- New package `internal/scaffold/` with these moves from
  `internal/initter/initter.go`:
  - `Action` struct → `scaffold.Action` (verbatim).
  - `printPlan(w, actions, dry)` → `scaffold.PrintPlan` (exported).
  - `mergeAgentsBlock(existing, tmpl)` → `scaffold.MergeAgentsBlock`.
  - `extractBlock(tmpl)` → `scaffold.ExtractBlock`.
  - The `<!-- docops:start -->` / `<!-- docops:end -->` marker
    constants → exported constants in `scaffold/`.
  - `dirAction(opts, rel)` → `scaffold.DirAction(rootAbs, rel)` —
    flatten the `Options` dependency to just the root path so
    upgrader can call it without an initter.Options.
  - `fileAction(opts, rel, body, mode)` → `scaffold.FileAction(rootAbs,
    rel, body, mode, force)` — same flattening; pass the force flag
    explicitly instead of reading it from an Options struct.
- New helper `scaffold.LoadShippedSkills() (map[string][]byte, error)`
  that wraps `templates.Skills()` and returns the same map. Both
  initter and upgrader call this.
- `internal/initter/` re-imports from `internal/scaffold/` and is
  reduced accordingly. The public surface of `initter` (the `Run`
  function, `Options`, `Result`) is unchanged.
- `cmd/docops/cmd_init.go` is **not modified**.
- Existing tests pass unchanged:
  - `go test ./internal/initter/...`
  - `go test ./cmd/docops/...` (`cmd_init_test.go` in particular).
  - `go test ./...` overall green.
- New file `internal/scaffold/scaffold_test.go` covers the moved
  helpers directly: `MergeAgentsBlock` round-trip, `ExtractBlock`
  edge cases (no markers, nested markers), `FileAction` skip vs
  overwrite vs force.

## Notes

The intent is to make the diff for TP-018 *only* show the genuinely
new upgrader code, not a copy of helpers initter already had. After
this lands, `internal/initter/initter.go` should be ~150 lines smaller
and `internal/scaffold/scaffold.go` should hold the lifted helpers.

Do not change `Options` field names or wire in any new behavior —
that is TP-018's job. If a helper resists clean extraction (say,
because it reads three fields off `initter.Options`), keep it in
initter for now and revisit during TP-018 implementation. Refactor
debt is fine; over-design is not.

The `Action.Kind` string set stays as-is (`"mkdir"`, `"write-file"`,
`"merge-agents"`, `"skip"`). TP-018 may add `"remove"` and
`"refresh"` — that addition is in scope for TP-018, not here.

The `templates/skills_lint_test.go` enumerates valid subcommands and
flags — it does not move; it stays in `templates/`. Only Go source
moves are in scope here.
