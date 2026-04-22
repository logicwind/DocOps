---
title: skills.sh-compatible layout — .agents/skills/ canonical + per-agent symlinks
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0022]
depends_on: [TP-007, TP-018]
---

# skills.sh-compatible layout — .agents/skills/ canonical + per-agent symlinks

## Goal

Update `docops init` and `docops upgrade` to adopt the skills.sh canonical
layout: skill files live in `.agents/skills/docops/`; tool-specific dirs
(e.g. `.claude/skills/docops`) become symlinks. Per ADR-0022.

## Acceptance

### init changes

- Agent detection: at init time, scan `~/.<agent>` directories to infer
  which agents the user has installed. Covered agents and their symlink
  targets (those whose `skillsDir ≠ .agents/skills/`):
  - `claude-code` → `.claude/skills/docops` → `../../.agents/skills/docops`
  - `windsurf` → `.windsurf/skills/docops` → `../../.agents/skills/docops`
  - Add others as the skills.sh registry grows; table lives in
    `internal/scaffold/agents.go`.
- Detection result presented in the TTY confirmation summary as a new
  "Symlinks" section (mirrors the existing "Files" section). Example:
  ```
  Symlinks
    + .claude/skills/docops → .agents/skills/docops
  ```
- `--agents <slug,...>` flag overrides auto-detection. Accepts
  comma-separated slugs from `internal/scaffold/agents.go`. Passing an
  unknown slug is a hard error with a list of valid slugs.
- If no agent requiring a symlink is detected (and `--agents` not passed),
  skip symlink creation silently — `.agents/skills/docops/` alone suffices
  for Cursor, Cline, Copilot, and most others.
- Symlinks are relative (not absolute) so the repo is portable across
  machines and containers.
- `docops init --dry-run` prints symlinks it would create but creates
  nothing.

### upgrade changes (depends on TP-018)

- Update `internal/upgrader/` to target `.agents/skills/docops/` as the
  canonical path instead of `.claude/skills/docops/`.
- One-time migration for v0.1.x projects (`.claude/skills/docops/` exists
  as a real directory, `.agents/skills/` absent):
  1. Copy `.claude/skills/docops/` → `.agents/skills/docops/`.
  2. Delete `.claude/skills/docops/` (directory).
  3. Create symlink `.claude/skills/docops` → `../../.agents/skills/docops`.
  4. Delete `.cursor/commands/docops/` if present (Cursor reads
     `.agents/skills/` directly; no symlink needed).
  Report each step with the standard sigils:
  ```
  + .agents/skills/docops/       (created — canonical)
  ~ .claude/skills/docops        (converted to symlink)
  - .cursor/commands/docops/     (removed — Cursor reads .agents/skills/)
  ```
- Idempotent: if `.agents/skills/docops/` already exists and
  `.claude/skills/docops` is already a symlink pointing there, the path is
  reported as `= .claude/skills/docops  (up to date)`.
- `.docops-manifest` (from TP-018) lives in `.agents/skills/docops/`, not
  in the symlink. Upgrade reads/writes it there.

### scaffold package

- Extract a new `internal/scaffold/agents.go` defining the agent registry:
  ```go
  type Agent struct {
      Slug       string
      SkillsDir  string   // relative to project root; "" means .agents/skills
      DetectDir  string   // e.g. "~/.claude" — empty means always-on
  }
  ```
- `NeedsSymlink(a Agent) bool` returns true when `SkillsDir != ""`.
- Used by both `internal/initter` and `internal/upgrader`.

### gitignore / templates

- Ensure `.gitignore.tmpl` does NOT exclude `.agents/` or `.claude/skills/`.
  Symlinks must be committed so clones work without running `docops init`.
- `templates/AGENTS.md.tmpl`: add one-line mention of `.agents/skills/` as
  the cross-tool skill directory so future init'd projects learn about it.

### tests

- `internal/scaffold/agents_test.go`:
  - All entries with a non-empty `SkillsDir` produce a valid relative path.
  - `claude-code` entry has `SkillsDir = ".claude/skills"` and
    `DetectDir = "~/.claude"`.
- `internal/initter/initter_test.go`:
  - Fresh init with `claude-code` detected: `.agents/skills/docops/` created
    as directory, `.claude/skills/docops` created as symlink pointing to
    `../../.agents/skills/docops`. Idempotent.
  - Fresh init with no agent detected: only `.agents/skills/docops/` created;
    no `.claude/` directory created.
  - `--agents claude-code` flag forces symlink even when `~/.claude` absent.
  - `--agents unknown-slug` exits 1 with error message listing valid slugs.
- `internal/upgrader/upgrader_test.go` (extends TP-018 tests):
  - Migration scenario: v0.1.x layout (`.claude/skills/docops/` real dir,
    `.cursor/commands/docops/` real dir) → after upgrade: `.agents/skills/docops/`
    real dir, `.claude/skills/docops` symlink, `.cursor/commands/` absent.
  - Idempotent on re-run: all `=`.
- `templates/skills_lint_test.go`: no changes needed for this task.

## Notes

The skills.sh CLI source (`npx skills` dist bundle) confirms:
- Canonical dir: `<cwd>/.agents/skills/<name>` (project) or
  `~/.agents/skills/<name>` (global).
- Claude Code `skillsDir` = `.claude/skills` → gets a symlink.
- Cursor `skillsDir` = `.agents/skills` → no symlink; reads canonical.
- Windsurf `skillsDir` = `.windsurf/skills` → gets a symlink.
- 20+ other agents also use `.agents/skills` directly.

Symlink creation in Go: use `os.Symlink(target, link)` with a relative
target computed via `filepath.Rel(filepath.Dir(link), target)`. On Windows,
`os.Symlink` requires Developer Mode or elevated privileges; if it fails,
fall back to copying and emit a warning: `  ! .claude/skills/docops  (copied
— symlink requires Developer Mode on Windows)`.

Do not auto-tag or auto-release as part of this task. TP-019 ships alongside
or just after TP-018, most likely in v0.1.2 or v0.2.0.
