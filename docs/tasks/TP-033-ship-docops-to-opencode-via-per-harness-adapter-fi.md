---
title: Ship /docops:* to OpenCode and Codex via per-harness adapter
status: done
priority: p2
assignee: claude
requires: [ADR-0028, ADR-0022, ADR-0013]
depends_on: []
---

## Shipped in v0.4.0

- Phase 1 (`cf3a2c7`) — `Harness` interface + registry; Claude/Cursor
  adapters. Byte-identical output.
- Phase 2 (`daa00b6`) — OpenCode adapter + layout-aware writer
  (`planHarness`, frontmatter parser/serializer, tool-name mapping).
- Phase 2b (`0807a1a`) — Codex adapter (`LayoutNestedSkillDir`),
  per-command `name:` injection, manifest tracks subdir names.
- Phase 3 (`5182e9b`) — harness detection; `--harnesses` /
  `--no-<slug>` flags; exported `DetectInstalledHarnesses` and
  `KnownHarnessSlugs`.
- Phase 4 (this commit) — README + CHANGELOG + v0.4.0 bump.

Not yet shipped: Kilo, Windsurf, Gemini, Copilot, Antigravity, Augment,
Trae. Each is a separate TP when demand shows up.


## Goal

Extend `docops upgrade` to ship `/docops:*` slash commands into **OpenCode and Codex** alongside the existing Claude Code and Cursor targets, via the per-harness adapter architecture defined in ADR-0028. These two are the first net-new harnesses; the adapter surface must be designed to accept more (Kilo, Windsurf, Gemini, Copilot, Antigravity, Augment, Trae) as demand appears.

Reference: GSD's installer (`get-shit-done/bin/install.js`) is the proven design. Port the shape, not the code.

- `copyFlattenedCommands` — flat-prefix writer used by OpenCode/Kilo (line 3365).
- `copyCommandsAsCodexSkills` — **nested-skill-dir writer** used by Codex (line 3427); each command becomes a directory `docops-<cmd>/` containing `SKILL.md`.
- `getDirName` / `getConfigDirFromHome` / `getOpencodeGlobalDir` — dir resolution (lines 132, 152, 183).
- Codex global dir: `--config-dir` > `CODEX_HOME` env var > `~/.codex` (lines 261–269).
- `claudeToOpencodeTools` / `convertToolName` — tool-name mapping (lines 622, 651).
- `convertClaudeToOpencodeFrontmatter` / `convertClaudeCommandToCodexSkill` — frontmatter transformers.
- `CODEX_AGENT_SANDBOX` constant (line 24) — Codex agents need a per-agent sandbox declaration (`workspace-write` / `read-only`) merged into `config.toml`. Docops ships no agents today, so this is informational only.
- `gsd-file-manifest.json` — per-harness manifest pattern.

## Acceptance

### Phase 1 — Adapter interface + refactor (no behavior change)

- [ ] Introduce a `Harness` type in `internal/upgrader/` (or a new `internal/harness/` package if it cleans the dep graph) with:
  - `Slug() string`
  - `LocalDir() string` — project-local target (e.g. `.opencode/command`)
  - `GlobalDir() (string, bool)` — user-level target if applicable; XDG-aware
  - `Layout() Layout` — enum `{NestedFile, FlatPrefixFile, NestedSkillDir}`. Codex needs the third form: each command is a directory `docops-<cmd>/` with a single `SKILL.md` inside, not a flat `.md` file. Design the writer to dispatch on this enum.
  - `FilenameFor(cmd string) string` — for `NestedFile` returns `"docops/get.md"`; for `FlatPrefixFile` returns `"docops-get.md"`; for `NestedSkillDir` returns `"docops-get/SKILL.md"`
  - `TransformFrontmatter(src map[string]any) (map[string]any, error)` — pure; no I/O
- [ ] Existing Claude and Cursor targets become adapters. `docopsSkillDirs()` is deleted; callers iterate the registry.
- [ ] `writeManifest` is invoked per harness dir (already per-dir today; just passes through the registry).
- [ ] All existing `docops upgrade` tests pass unchanged. No output diff on a Claude/Cursor project.

### Phase 2 — OpenCode adapter

- [ ] `OpenCodeAdapter` registered with:
  - `LocalDir` = `.opencode/command`
  - `GlobalDir` honours `OPENCODE_CONFIG_DIR`, `OPENCODE_CONFIG` (dirname), `XDG_CONFIG_HOME/opencode`, else `~/.config/opencode`. Mirrors GSD's `getOpencodeGlobalDir` precedence.
  - Flat-prefix filenames — `docops-<cmd>.md`
  - Transformer:
    - Drop `name:` (filename is the ID)
    - Convert `allowed-tools:` list → `tools:` map with `true` values
    - Apply Claude→OpenCode tool-name map: `AskUserQuestion→question`, `SlashCommand→skill`, `TodoWrite→todowrite`, `WebFetch→webfetch`, `WebSearch→websearch`; MCP tools (`mcp__*`) preserved verbatim; everything else lowercased
    - Preserve `description`, `argument-hint`, and any other benign keys untouched
- [ ] Golden-file test: fixture command (say, `templates/commands/docops/get.md`) through the OpenCode transformer produces the exact expected file. Committed under `internal/upgrader/testdata/opencode/`.
- [ ] Round-trip test: tools set equality modulo the mapping (no tool silently dropped).
- [ ] `docops upgrade` on a repo with `~/.config/opencode/` present writes `.opencode/command/docops-*.md` for every shipped command.
- [ ] Manifest `.opencode/command/.docops-manifest` lists the written files so the next upgrade can clean removed ones.

### Phase 2b — Codex adapter

- [ ] `CodexAdapter` registered with:
  - `LocalDir` = `.codex/skills`
  - `GlobalDir` precedence: `--config-dir` flag (if docops supports one) > `CODEX_HOME` env var > `~/.codex`. Mirrors GSD lines 261–269.
  - Layout = `NestedSkillDir` — emits `docops-<cmd>/SKILL.md`
  - Transformer (`convertClaudeCommandToCodexSkill` equivalent):
    - Rewrites the Claude command frontmatter into a Codex skill frontmatter (`name:`, `description:` — see GSD's source for the exact shape).
    - Path rewrites: `~/.claude/` / `./.claude/` / `$HOME/.claude/` inside command bodies rewritten to the Codex equivalent.
    - MCP tools preserved; other tools translated per Codex's conventions (inspect GSD's helper at line 3427+ for the authoritative list).
- [ ] Remove stale Codex skills before write: any directory under `.codex/skills/` whose name starts with `docops-` is removed first (matches GSD's idempotent-install behaviour).
- [ ] Golden-file test per shipped command under `internal/upgrader/testdata/codex/docops-<cmd>/SKILL.md`.
- [ ] `docops upgrade` on a repo with `~/.codex/` (or `CODEX_HOME`) present writes the nested-skill tree.
- [ ] Manifest for Codex: `.codex/skills/.docops-manifest` lists the directory names owned (e.g. `docops-get`, `docops-audit`) — not individual SKILL.md paths — so cleanup is a per-directory `rm -rf`.
- [ ] Decision: do not touch `~/.codex/config.toml`. `CODEX_AGENT_SANDBOX` is an agent-only concern; docops ships no agents. Call this out in release notes.

### Phase 3 — Harness detection + flags

- [ ] Default behaviour: write to every harness whose local dir exists in the project *or* whose global dir exists on the machine. Detection table:
  - Claude — `~/.claude/` or `./.claude/` exists
  - Cursor — `~/.cursor/` or `./.cursor/` exists
  - OpenCode — `$OPENCODE_CONFIG_DIR`, dirname(`$OPENCODE_CONFIG`), `$XDG_CONFIG_HOME/opencode`, or `~/.config/opencode/` exists; or `./.opencode/` exists
  - Codex — `$CODEX_HOME` env set, or `~/.codex/` exists; or `./.codex/` exists
- [ ] `docops upgrade --harnesses claude,opencode,codex` — explicit list overrides detection.
- [ ] `docops upgrade --no-opencode` / `--no-codex` / `--no-cursor` — opt out per-harness.
- [ ] `docops init` propagates the same flags when scaffolding first.
- [ ] `docops upgrade --dry-run` output lists each target dir it would write and why (detected / flagged / missing).

### Phase 4 — Docs + release

- [ ] `README.md`, `AGENTS.md.tmpl`, `CLAUDE.md.tmpl` — any line that mentions `.claude/commands/docops/` as *the* target gets reframed as "the Claude target; see `docops upgrade --harnesses` for OpenCode / Cursor / …".
- [ ] `CLAUDE.md.tmpl` Orientation block — mention `/docops-*` as the OpenCode-dialect invocation alongside `/docops:*`.
- [ ] CHANGELOG entry under v0.4.0 (minor bump — new user-visible targets).
- [ ] Release notes call out: OpenCode users running `docops upgrade` will see `.opencode/command/docops-*.md` appear for the first time; nothing is removed.

## Non-goals

- **Skills are untouched.** ADR-0022's `.agents/skills/` + symlink model is unchanged. This work only covers slash commands.
- **No agent/subagent shipping.** docops ships no sub-agents today (it's a CLI, not an agentic framework). The adapter does not need an `AgentsDir()` method; add it later if needed.
- **No hook/settings injection.** GSD merges entries into `opencode.json` / `claude settings.json` at install time for its own hooks. docops ships no hooks; skip this layer.
- **Additional harnesses beyond OpenCode and Codex are out of scope** for this TP. Ship the interface, ship both adapters; track each additional harness (Kilo, Windsurf, Gemini, Copilot, Antigravity, Augment, Trae) as its own TP when demand shows up.
- **No `.agents/commands/` canonical store for slash commands.** Considered and rejected in ADR-0028. Each harness gets its own translated copy written directly into its expected dir. Skills continue to use the `.agents/skills/` + symlink model from ADR-0022 — that layer is unchanged.

## Notes

- `internal/upgrader/upgrader.go:116` is the single point of change today. The adapter refactor deletes that hardcoded list.
- Cursor's `.cursor/commands/docops/` predates the skills.sh era (see ADR-0022 context). During Phase 1, verify that Cursor still reads slash commands from that path. If Cursor has moved, fix the Cursor adapter in the same PR — don't carry dead paths forward.
- Lint test analogous to `templates/skills_lint_test.go`: for each harness × each shipped command, assert (a) the transformed file parses under the harness's expected shape, (b) the tool set round-trips, (c) no key silently dropped.
- Manifest format — keep the plain-text one-file-per-line format already at `.claude/commands/docops/.docops-manifest`. GSD uses JSON with SHA-256s; nice-to-have, not required.

## Phase ordering

1. Interface + refactor (Phase 1) — self-contained, mergeable on its own. The `Layout` enum must be designed up front to accommodate Codex's `NestedSkillDir`.
2. OpenCode adapter + tests (Phase 2) — depends on Phase 1.
3. Codex adapter + tests (Phase 2b) — depends on Phase 1, independent of Phase 2.
4. Detection + flags (Phase 3) — depends on Phases 2 and 2b.
5. Docs + release (Phase 4) — ships with Phases 2, 2b, and 3 in one version bump.

Phases 2 and 2b can land in parallel — different harness, different test fixtures, same interface. Split into PRs if review feels heavy: (PR 1) Phase 1 alone, (PR 2) Phase 2 (OpenCode), (PR 3) Phase 2b (Codex), (PR 4) Phase 3 + 4.
