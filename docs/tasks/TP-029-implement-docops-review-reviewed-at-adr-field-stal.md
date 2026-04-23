---
title: Implement docops review — reviewed_at ADR field + stale-review detection
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0026]
depends_on: []
---

## Goal

Ship the CLI + schema work defined in ADR-0026 so that ADRs whose
implementation has flipped to `done` can be review-marked against the
shipped code.

## Acceptance

- `docs/.docops/schema/decision.schema.json` accepts an optional
  `reviewed_at` field of format `date` (YYYY-MM-DD). Regenerated from
  `docops.yaml` via `docops schema`.
- `docops review` (no args) lists ADRs where
  `implementation == done` AND (`reviewed_at` missing OR
  `reviewed_at < max(commit_date)` for commits touching files
  referenced by the ADR's citing tasks). Supports `--json`.
- `docops review <ADR-ID>` prints the ADR body plus the last N commits
  (default 20, configurable via `--since` or `--limit`) touching files
  referenced by its citing tasks. Commits must include subject + hash +
  date; file list optional behind a flag to keep default output short.
- `docops review <ADR-ID> --mark` writes today's date into
  `reviewed_at` via atomic frontmatter rewrite (reuse the same write
  path as `docops new`). Idempotent: re-running same day is a no-op.
- `docs/STATE.md` surfaces a `stale_review` row in the
  needs-attention table when the count is >0. Zero count stays silent.
- Tests:
  - Schema acceptance of `reviewed_at` as optional date.
  - Stale detection: ADR with `implementation=done`, `reviewed_at`
    older than a citing-task commit → flagged. Newer `reviewed_at` →
    not flagged.
  - `--mark` idempotency on same-day runs.
  - STATE.md row emitted iff count > 0.

## Notes

- Detecting "files referenced by citing tasks" needs a convention. The
  simplest: scan task body for filesystem-looking strings
  (`internal/foo.go`, `templates/...`) and use those as the git-log
  path filter. Fall back to "any commit since the last citing-task
  done-date" when no paths resolve. This heuristic can be tightened
  later.
- Do not add `reviewed_at` to CTX or TP — review-delta is ADR-specific.
- Keep command output stable enough to diff in tests; no color codes
  when stdout is not a TTY.

