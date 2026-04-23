---
title: Write CLAUDE.md alongside AGENTS.md in init and upgrade
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0024]
depends_on: []
---

## Goal

Implement ADR-0024: docops writes both `CLAUDE.md` and `AGENTS.md`
during `init` and refreshes both during `upgrade`. Both files share
the same `<!-- docops:start --> … <!-- docops:end -->` block.

## Acceptance

### Templates

- New `templates/CLAUDE.md.tmpl` — same docops block as
  `AGENTS.md.tmpl`, with a one-line non-block preamble that mentions
  `AGENTS.md` is the multi-tool sibling.
- New `templates.ClaudeBlock()` accessor in `templates/templates.go`
  that returns the embedded `CLAUDE.md.tmpl` bytes (mirrors
  `templates.AgentsBlock()`).
- New test `templates/agents_claude_block_sync_test.go`: asserts that
  `scaffold.ExtractBlock(AgentsBlock())` equals
  `scaffold.ExtractBlock(ClaudeBlock())` byte-for-byte. Catches drift
  the moment a templates author edits one without the other.

### initter

- `internal/initter/initter.go` `plan()` calls a generalized
  `planMarkdownBlock(opts, "CLAUDE.md", claudeTmpl)` and appends both
  the AGENTS.md and CLAUDE.md actions. Existing `planAgents` becomes
  the type-generic helper or is renamed; behavior for AGENTS.md is
  unchanged.
- The init announcement block in `cmd/docops/cmd_init.go` mentions
  CLAUDE.md alongside AGENTS.md.

### upgrader

- `internal/upgrader/upgrader.go` `plan()` emits a refresh action for
  both `CLAUDE.md` and `AGENTS.md` when each file is present (or
  creates either if absent — the planner already handles missing
  files via the same merge logic).
- The upgrade announcement block in `cmd/docops/cmd_upgrade.go`
  mentions both files.

### Tests

- `internal/initter/initter_test.go` adds a CLAUDE.md case mirroring
  the existing AGENTS.md merge test.
- `internal/upgrader/upgrader_test.go` adds:
  - A case where both files exist with stale blocks → both refresh.
  - A case where AGENTS.md exists with the block but CLAUDE.md is
    absent → CLAUDE.md is created with the full template.
  - A case where the user has hand-written content in CLAUDE.md
    outside the block → preserved across upgrade.

### Docs and fallout

- `templates/AGENTS.md.tmpl` "Notes for humans" footer references
  CLAUDE.md so users learn both files are managed.
- `templates/skills/docops/init.md` and
  `templates/skills/docops/upgrade.md` mention CLAUDE.md.
- README "Quickstart" briefly notes that init scaffolds both files.
- AGENTS.md.tmpl mention of `docops upgrade` already covers the
  upgrade path — no new line needed.

## Notes

The block-merge logic in `internal/scaffold.MergeAgentsBlock` is
already file-agnostic — it operates on `existing []byte` and returns
merged bytes. No changes needed there. The "AGENTS.md" name is
plumbed only at the planner-call layer, so generalization is small.

For the docops repo itself: after this ships, run `docops upgrade
--yes` once to scaffold the missing CLAUDE.md (the repo currently
has only AGENTS.md). That can be a separate dogfood commit.

Out of scope: `.cursorrules`, `.aider.conf.yml`, host packaging,
template rewriting. ADR-0024 explicitly limits scope to the two
markdown files. A future ADR can expand if needed.
