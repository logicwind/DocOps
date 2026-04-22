---
title: Implement `docops new <type>` — scaffold documents with ID allocation
status: backlog
priority: p1
assignee: unassigned
requires: [ADR-0003, ADR-0002, ADR-0004]
depends_on: [TP-002]
---

# Implement `docops new <type>` — scaffold documents with ID allocation

## Goal

Commands that create new CTX, ADR, or Task documents with valid frontmatter, atomic ID allocation, and correct file naming.

## Acceptance

- `docops new ctx "title" [--type <type>]` creates `docs/context/CTX-NNN-slugified-title.md`.
- `docops new adr "title" [--related ADR-0010,ADR-0004]` creates `docs/decisions/ADR-NNNN-slugified-title.md`.
- `docops new task "title" --requires ADR-0020,CTX-003 [--priority p1] [--assignee claude]` creates `docs/tasks/TP-NNN-slugified-title.md`.
- `docops new task` refuses to run without at least one `--requires` citation (enforces ADR-0004 at creation time, not just at lint time).
- IDs are allocated atomically from a counter file (`docs/.docops/counters.json`) with file-locking to prevent collisions on parallel runs.
- The newly created file opens in `$EDITOR` by default unless `--no-open` is set.
- Every field in the scaffolded frontmatter is pre-filled to valid defaults; the author only edits body content.
- `--json` flag returns `{id, path}` for scripting.

## Notes

The atomicity of ID allocation matters. Test parallel invocation explicitly — two concurrent `docops new task` calls must never produce the same ID.

Slugification rules: lowercase, hyphens, ASCII-only, truncate at ~50 chars.
