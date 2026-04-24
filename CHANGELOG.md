# Changelog

All notable changes to docops are recorded here. Dates are UTC.

## v0.4.0 — 2026-04-24

### Added — multi-harness slash-command delivery (TP-033, ADR-0028)

`docops upgrade` now ships `/docops:*` slash commands into OpenCode and
Codex in addition to Claude Code and Cursor. Each harness gets files
translated into its own YAML dialect — frontmatter is rewritten per
target, not symlinked.

Supported harnesses:

| Harness     | Local dir                   | Invocation      | Layout                          |
|-------------|-----------------------------|-----------------|---------------------------------|
| Claude Code | `.claude/commands/docops/`  | `/docops:get`   | nested files                    |
| Cursor      | `.cursor/commands/docops/`  | `/docops:get`   | nested files                    |
| OpenCode    | `.opencode/command/`        | `/docops-get`   | flat-prefix (`docops-get.md`)   |
| Codex       | `.codex/skills/docops-*/`   | `docops-get`    | nested skill dirs (`SKILL.md`)  |

### Added — harness detection + new flags

- `docops upgrade` auto-detects installed harnesses. A harness is
  written to when its project-local dir exists *or* its user-level dir
  exists (`~/.claude/commands`, `~/.cursor/commands`, OpenCode XDG path,
  `$CODEX_HOME` or `~/.codex/skills`).
- `--harnesses claude,opencode` — pin the target list explicitly
  (overrides detection).
- `--no-claude` / `--no-cursor` / `--no-opencode` / `--no-codex` —
  subtract one harness from the detected/pinned set.
- `DetectInstalledHarnesses(root)` and `KnownHarnessSlugs()` are
  exported from `internal/upgrader` for library callers.

### Behavior change

Previously `docops upgrade` wrote to *every* harness dir unconditionally
(even if you had none of those tools installed). Starting in v0.4.0,
`docops upgrade` only writes to harnesses whose local or global dir
exists on your machine. Users who want the old "write everywhere"
behavior can pass `--harnesses claude,cursor,opencode,codex`.

Newly-appearing harnesses (e.g. you install OpenCode after running
`docops upgrade`) will show up on the next `docops upgrade` with no
further action. Nothing is removed from existing installs.

### Internal

- New `Harness` interface with `Layout` enum (`LayoutNestedFile`,
  `LayoutFlatPrefixFile`, `LayoutNestedSkillDir`) — adding a new
  harness is now ~50 LoC + a golden-file fixture.
- Writer `planSkillDir` renamed to `planHarness` and dispatches on
  Layout. Each layout has its own planner and manifest semantics.
- Minimal YAML frontmatter parser/serializer in
  `internal/upgrader/frontmatter.go` — pure, deterministic, handles
  the subset docops commands need (strings, lists, maps).

### Unchanged (on purpose)

- Skills (`.agents/skills/…`) continue to use the symlink model from
  ADR-0022 — only slash commands got the per-runtime translation.
- The on-disk output for Claude Code and Cursor is **byte-identical**
  to v0.3.0 (regression-tested).
- No hooks or config-file merges (GSD's installer writes to
  `opencode.json` / Codex `config.toml` for its own hooks + agent
  sandboxing; docops ships no hooks or agents, so it skips that layer).
