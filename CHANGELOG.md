# Changelog

All notable changes to docops are recorded here. Dates are UTC.

## v0.5.0 — 2026-04-24

### Added — `docops html` and `docops serve` (TP-030, TP-031, ADR-0027)

A browsable HTML viewer for DocOps repositories. Two new CLI subcommands:

| Command | What it does |
|---|---|
| `docops html` | Emits `docs/.html/` containing just two files — `index.html` (the SPA) and `index.json` (a bundle with the enriched index + every doc body + STATE.md). Open the HTML file directly or deploy it to any static host. |
| `docops serve` | Starts a localhost web viewer (default `:8484`). Rebuilds the bundle in-memory on every request so the browser always shows the latest state. `--open` opens the default browser on startup. |

The viewer itself is a single-page app that loads once and navigates
client-side. Features:

- **Sidebar** — CTX / ADR / TP grouped tree with collapsible sections,
  status badges, search box, and current-doc highlight.
- **Right pane** — breadcrumb, frontmatter table, reverse-edge chips
  (Referenced by, Superseded by, Derived ADRs, Active tasks, Blocks),
  rendered markdown body. All `ADR-n` / `CTX-n` / `TP-n` tokens in the
  body are auto-linkified to their detail views.
- **Graph tab** — pinned column layout: CTX in 1 column on the left,
  ADR in 2 columns in the middle, TP in 3 columns on the right.
  Column-major fill keeps IDs in numerical order. Hover a node to
  focus its neighborhood (everything else fades); single-tap pins the
  focus; double-tap opens the doc. Edge colors by type: `supersedes`
  red, `requires` blue, `depends_on` purple, `related` gray.
- **Home view** — STATE.md + per-kind count tiles.
- **Hash routing** — `#/CTX/CTX-001`, `#/ADR/ADR-0027`, `#/TP/TP-030`,
  `#/state`, `#/graph`. Deep-links from terminal output or chat work.

### Design choices

- **Zero new Go dependencies.** Markdown rendering (`marked`), styling
  (Tailwind play CDN), and graph layout (`cytoscape.js`) all load from
  jsDelivr on first view; the browser caches them.
- **Binary delta is tiny** — one embedded HTML file (~20 KB) plus
  ~80 lines of Go for each of `cmd_html` / `cmd_serve`. No goldmark,
  no `html/template`, no fsnotify.
- **Read-layer consumer.** Both subcommands call `internal/index.Build`
  — the same code path as `docops index` / `get` / `graph` — so the
  viewer never reads `.index.json` directly (honors ADR-0018).

### Fixed

- `templates/CLAUDE.md.tmpl` had drifted from `AGENTS.md.tmpl` on the
  `docops list` flag hint (`--type ctx|adr|task` vs. the correct
  `--kind CTX|ADR|TP`). `TestAgentsClaudeBlocksInSync` now green.

### Internal

- New `internal/htmlviewer/` package: `spa.go` (embedded HTML),
  `bundle.go` (`BuildBundle` — index + bodies + state as one JSON),
  `emit.go` (static emitter), `serve.go` (HTTP handler).
- SPA exposes `window.__docopsCy` as an escape hatch for devtools and
  end-to-end tests.
- `docs/.html/` added to the project `.gitignore`.

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
