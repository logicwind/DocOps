---
title: Multi-harness slash-command delivery via per-runtime translation
status: draft
coverage: required
date: "2026-04-24"
supersedes: []
related: [ADR-0013, ADR-0021, ADR-0022]
tags: []
---

## Context

TP-023 (commit `379ee51`) moved `/docops:*` from skills to **slash commands** under `.claude/commands/docops/`. ADR-0022 had already solved multi-tool *skills* via a canonical `.agents/skills/` store + per-tool symlinks. Slash commands cannot follow that pattern: each harness reads its own frontmatter dialect, so the *file contents* differ per harness — not just the path.

Dialect differences observed in production (sourced from GSD's installer):

| Harness | Local dir | Global dir | Filename scheme | Tools shape |
|---|---|---|---|---|
| Claude Code | `.claude/commands/docops/` | `~/.claude/commands/docops/` | nested — `docops/get.md` → `/docops:get` | `allowed-tools:` list |
| OpenCode | `.opencode/command/` | `$XDG_CONFIG_HOME/opencode/command/` → `~/.config/opencode/command/` | flat prefix — `docops-get.md` → `/docops-get` | `tools:` map (`read: true`, …) |
| Codex | `.codex/skills/` | `$CODEX_HOME` → `~/.codex/skills/` | **nested skill dirs** — `docops-get/SKILL.md` | Codex skill dialect; agent sandbox map (`workspace-write`/`read-only`) in `config.toml` |
| Cursor | `.cursor/commands/docops/` | `~/.cursor/commands/docops/` | nested | Cursor slash-command dialect |
| Kilo | `.kilo/` | `$XDG_CONFIG_HOME/kilo/` | flat | `permission:` object with ordered keys |
| Antigravity | `.agent/` (singular — this harness only) | `~/.gemini/antigravity/` | per-harness | per-harness |
| Windsurf / Copilot / Gemini / Augment / Trae | per-harness | per-harness | mix | per-harness |

**Folder-naming confusion (resolved for the record):**

- **`.agent/`** (singular) is **Antigravity's** local dir. Nothing else uses it.
- **`.agents/skills/`** (plural, with `/skills/`) is the **skills.sh ecosystem canonical store** — used only for *skills*, not slash commands. Cursor, Cline, Copilot, Gemini CLI, Kilo, Roo, and Codex's skill reader all consume `.agents/skills/` directly. ADR-0022 already puts docops skills there and symlinks out to `.claude/skills/` etc.
- **`.codex/`** is Codex's own local dir (no relation to `.agent/`). Codex expresses commands **as skills**: each command is a directory `docops-<cmd>/` containing a `SKILL.md` file, not a flat `.md`.
- There is **no cross-harness canonical store for slash commands.** Each harness has its own dir *and* its own YAML dialect, so a single file can't serve multiple harnesses even if we put it under `.agents/commands/`.

Tool-name mappings also differ: `AskUserQuestion → question` (OpenCode), `Read → read_file` (Gemini), `Bash → bash` (Kilo permission set), MCP tools (`mcp__*`) preserved verbatim everywhere.

Today (v0.3.0): `internal/upgrader/upgrader.go:116` has a hardcoded two-entry list — `{".claude/commands/docops", ".cursor/commands/docops"}` — and writes canonical files verbatim into both. No translation. OpenCode users see zero `/docops:*` commands because the Claude frontmatter is invalid for OpenCode's loader.

GSD (reference: `get-shit-done/bin/install.js`, ~6300 LoC) has solved the same problem for 11 harnesses. Its model: per-runtime adapter with (a) dir map, (b) filename scheme, (c) frontmatter transformer, (d) XDG-aware global/local resolver, (e) per-harness manifest. That's the shape we need.

## Decision

Introduce a **per-harness adapter registry** in the installer. Each adapter declares:

1. **Slug** — `claude`, `opencode`, `cursor`, …
2. **Directory resolver** — local (`.opencode/command/`) and global (`$XDG_CONFIG_HOME/opencode`, falling back to `~/.config/opencode/`).
3. **Filename scheme** — `nested` (Claude, Cursor) or `flat-prefix` (OpenCode, Kilo, …). Determines whether `get.md` lands at `docops/get.md` or `docops-get.md`.
4. **Frontmatter transformer** — pure function that rewrites the canonical (Claude) frontmatter into the harness dialect. Includes a per-harness tool-name map.
5. **Manifest policy** — every harness dir gets its own `.docops-manifest` so `docops upgrade` can clean stale files on removal.

Canonical source stays in `templates/commands/docops/*.md` in Claude format. `docops upgrade` walks the harness registry, transforms, writes. `.agents/skills/` remains the store for *skills* per ADR-0022 — this ADR only governs slash commands.

**Harness detection:** default to every harness whose global or local config dir exists (`~/.config/opencode/`, `.opencode/`, `~/.cursor/`, …). Override via `docops upgrade --harnesses claude,opencode` or disable per-slug (`--no-opencode`).

**Shipping order:** interface + refactor Claude/Cursor to adapters (no behavior change) → OpenCode adapter + Codex adapter (the two harnesses with enough user demand to justify inclusion now) → demand-driven additions (Kilo, Windsurf, Gemini, Copilot, Antigravity, Augment, Trae) as each shows up in the wild.

**On the `.agents/` canonical-store idea (rejected for slash commands):**

The skills.sh pattern (ADR-0022) works because every consumer reads the *same* skill format. Slash commands don't have that property — Claude's `allowed-tools:` list, OpenCode's `tools:` map, and Kilo's `permission:` object are **mutually incompatible**; a single file cannot validate against all three schemas. We considered writing *translated* copies into `.agents/commands/docops/<harness>/` and symlinking from each harness's expected dir, but that adds indirection and an extra tree without solving anything the direct per-harness-write doesn't already solve — the translated files are no less numerous, and symlink fragility (Windows, case-insensitive filesystems, git worktrees) becomes a new failure mode. GSD considered this too and landed on direct per-harness writes; we follow suit.

## Rationale

- **Symlinks don't work** for slash commands — contents differ, not just paths.
- **GSD's model is proven** at production scale across 11 harnesses. Porting the pattern is cheaper than inventing one.
- **Pure translators** are trivially testable (golden-file per harness, round-trip equivalence).
- **Registry pattern** caps the blast radius of adding a harness to ~50 LoC + one test fixture.
- Keeps a **single source of truth** (`templates/commands/docops/`) so command authors write once.

## Consequences

- `internal/upgrader/` gains a `Harness` type (or a sibling `internal/harness/` package). Existing hardcoded list becomes two adapters.
- `docops upgrade` output grows: on a machine with OpenCode installed, it will write `.opencode/command/docops-*.md` where previously nothing was written. Users who install docops after OpenCode will see `/docops-*` auto-appear.
- Need a lint test analogous to `templates/skills_lint_test.go` that asserts each harness's transformer output against a fixture.
- Every new harness is a new adapter; each adapter is a new source of drift risk. Mitigate with golden-file tests committed alongside the adapter.
- ADR-0022 (`.agents/skills/` symlink model) is unchanged for skills. This ADR only covers slash commands, which live in a separate, per-harness dir.
- Cursor path `.cursor/commands/docops/` predates the skills.sh era — confirm it's still the correct Cursor slash-command target during the refactor. If Cursor has moved, fix in this work.
- Codex's nested-skill filename scheme (`docops-<cmd>/SKILL.md`) is different enough from the others that the `Harness` interface must accommodate both single-file and directory-per-command output. Design the interface with that in mind from the start.
