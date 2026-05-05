# Changelog

All notable changes to docops are recorded here. Dates are UTC.

## Unreleased

### Added — Beta release channel (ADR-0032, TP-042)

Opt-in prerelease channel published as a parallel Homebrew formula and Scoop
manifest in the existing `logicwind/homebrew-tap` and `logicwind/scoop-bucket`
repos. Channel routing is driven by templated `skip_upload` in
`.goreleaser.yml` keyed on the SemVer prerelease bit — stable releases bump
only `Formula/docops.rb` / `bucket/docops.json`; prerelease tags
(`vX.Y.Z-beta.N`, `-alpha.N`, `-rc.N`) bump only `Formula/docops@beta.rb` /
`bucket/docops-beta.json`.

```sh
brew install logicwind/tap/docops@beta   # macOS / Linux
scoop install docops-beta                # Windows
```

Scoop has no `@channel` convention, so the parallel manifest uses `-beta`.

## v0.6.0 — 2026-04-30

### Added — Amendments as first-class decision metadata (ADR-0025)

ADRs can now carry a structured, append-only `amendments:` log for
editorial fixes, errata, clarifications, and late-binding patches that
don't warrant a full superseding ADR. Validator, CLI, index, STATE.md,
and the static HTML viewer are all amendment-aware.

```yaml
# docs/decisions/ADR-0019-...md
amendments:
  - date: 2026-04-23
    kind: editorial            # editorial | errata | clarification | late-binding
    by: nix
    summary: "Tap/bucket repo names: per-tool → org-wide convention"
    affects_sections: ["v0.1.0 scope"]
    ref: TP-024
```

- **Schema + validator** — `kind` enum (4 values) is the single source
  of truth for both the Go validator and `decision.schema.json`. Inline
  `[AMENDED YYYY-MM-DD kind]` markers in the body are correlated with
  frontmatter entries; mismatches are validation errors. Markers inside
  fenced code blocks are skipped. Amendments on `superseded` ADRs emit
  warnings rather than errors.
- **`docops amend` CLI** — non-interactive mutation. Mirrors ADR-0025's
  flag surface (`--kind`, `--summary`, `--section`, `--ref`, `--by`,
  `--body`/`--body-file`, `--marker-at`). yaml.Node-based frontmatter
  edits preserve comments, key order, and quoting on unrelated fields.
  Atomic tmp+rename write.
- **Index + STATE.md** — `docs/.index.json` gains `amendments` per ADR
  plus a top-level `recent_amendments` list (newest-first, windowed by
  `recent_activity_window_days`, UTC-midnight comparison).
  STATE.md gains a "Recent amendments" section.
- **Static viewer (`docops html` / `docops serve`)** — ADR detail pages
  render an Amendments section under the body; the Home view shows a
  Recent amendments panel after STATE.md. The viewer bundle now carries
  `recent_amendments` in addition to per-doc `amendments`.
- **TP-027 backfill** — ADR-0019's HTML-comment amendment stub is
  promoted to a proper frontmatter entry.

Audit rules from ADR-0025 (≥5 amendments threshold, hand-edit drift,
stale-ref) are deferred to TP-039.

### Changed — Slash command surface narrows to 5 milestone moments (ADR-0029)

Slash-style harnesses (Claude, Cursor, OpenCode) now ship a focused set
of `/docops:*` commands instead of one slash per CLI verb:

```
init      progress      next      do      plan
```

Granular operations (`get`, `list`, `graph`, `search`, `audit`, `close`,
`new-adr`, `new-ctx`, `new-task`, `refresh`, `state`, `upgrade`) remain
available as **skills** for natural-language dispatch by the LLM, and
as CLI verbs. The `/docops:do` skill routes free-form intents to the
right skill or CLI invocation.

`docops upgrade` removes the 12 deprecated slash files from
`.claude/commands/docops/` and `.cursor/commands/docops/` automatically
on next run. **Codex bundle is unchanged** — it uses skill-bundle
delivery (not slashes), so the full surface stays in-bundle as
subroutines.

### Added — ADR-0030 (draft) — named baselines

Drafted but not implemented: a baseline is a name + git tag + frozen
index pointer (`docs/baselines/<name>.json`). Future work will add
`docops baseline create|list|show|diff|current` and
`docops get <ID> --at <baseline>`. No code change in this release.

### Changed — Status enum literals surfaced where LLMs read

LLMs were guessing `in_progress`, `wip`, `todo` for task status and
hitting validator errors. The canonical enums are now inline in the
docops block in `AGENTS.md`/`CLAUDE.md` (and templates), in the
`new-task`, `new-adr`, and `close` skill files, with the common wrong
guesses called out. JSON Schema remains canonical; these are read-side
hints to short-circuit the trial-and-error loop.

The `new-task` skill no longer references the nonexistent
`docops status TP-xxx active` command — replaced with explicit
edit-frontmatter + `docops refresh`.

### Changed — CI runtimes bumped to Node 24

`actions/checkout v4 → v6`, `actions/setup-go v5 → v6`,
`goreleaser/goreleaser-action v6 → v7` to clear GitHub's 2026-06-02
Node 20 deprecation.

### Internal

- New `internal/amender/` package; `cmd/docops/cmd_amend.go`.
- `schema.Amendment` + `ADR.Amendments` (yaml `omitempty`); validator
  gains `ValidateAmendmentMarkers`; `loader.Doc` gains `Body []byte`
  for ADRs so the validator can correlate markers.
- `index.IndexedDoc.Amendments`, `index.Index.RecentAmendments`,
  `index.IndexedAmendment`, `index.RecentAmendment`.
- `state.Snapshot` threads `RecentAmendments` through; renderer emits
  the section only when non-empty.
- `htmlviewer.Bundle.RecentAmendments` (was silently dropped).
- `scaffold.SlashDeliverableCmds` defines the milestone-moment subset;
  upgrader auto-removes deprecated slash files via the existing
  "no-longer-shipped" cleanup path. New
  `TestRun_DeprecatesPreADR0029Slashes` covers the migration.
- `templates/skills/docops/do.md` routing table updated to skill names
  (or CLI fallback) rather than defunct slashes.
- `skill-lint` allowlist gains `amend`.

### Known gaps (tracked)

- TP-035 — `/docops:do` dispatcher fixture suite (≥95% routing
  accuracy bar). Load-bearing under ADR-0029 long-term; ships shortly
  after.
- TP-037 — Timeline view in static HTML viewer.
- TP-038 — Graph node annotations (amended/draft/stale).
- TP-039 — Deferred amendment audit rules from ADR-0025.
- ADR-0030 implementation — pending design ideation.
- TP-034 deferred behavior — "preserve user-modified slash files with
  warning" rather than always overwriting on upgrade.

### Migration

Pre-launch — no migration needed. If you have an in-flight DocOps repo,
running `docops upgrade` will:

1. Remove 12 deprecated `/docops:*` slash files from Claude/Cursor
   command directories.
2. Refresh `AGENTS.md` / `CLAUDE.md` docops blocks with Invariant #6
   (status enums).

ADRs without `amendments:` continue to validate; the field is additive.

## v0.5.2 — 2026-04-25

### Changed — Codex layout collapses to one skill bundle

`docops upgrade` now writes Codex's docops surface as a single
**skill bundle** instead of 17 separate per-command skills:

```
# Before (v0.4.x – v0.5.1)
.codex/skills/
  docops-audit/SKILL.md
  docops-close/SKILL.md
  docops-get/SKILL.md
  ... (17 separate top-level skills)

# After (v0.5.2)
.codex/skills/docops/
  SKILL.md          ← bundle entry: auto-loaded by description match
  audit.md          ← per-subroutine files
  close.md
  get.md
  ... (17 subroutines under one skill)
```

The original layout misread Codex's auto-trigger model. Codex picks
skills by description matching, so 17 narrow descriptions
("get a doc", "close a task", …) competed with each other instead of
one cohesive `docops` skill describing the whole tool surface.
Aligns with how every other Codex skill (`agforge`, `screenshot`,
GSD's bundled skills) is structured. See ADR-0028 amendment.

**No migration needed** if you're not yet on docops — pre-launch.
If you have v0.4.x or v0.5.x with the old Codex layout, the next
`docops upgrade` removes the 17 stale `docops-*` directories and
writes the bundle. Other harnesses (Claude, Cursor, OpenCode) are
unchanged — those use slash-command models, not skills.

### Internal

- New `LayoutSkillBundle` enum value in `internal/upgrader/`
  replaces `LayoutNestedSkillDir`. The `Codex` adapter now uses it.
- `templates/skills/docops/SKILL.md` shipped as a new template; it
  is the bundle's entry-point and bypasses the per-harness frontmatter
  transform.
- `planSkillBundleHarness` replaces `planNestedSkillDirHarness` in
  the upgrader.

## v0.5.1 — 2026-04-24

### Fixed

- **`make release VERSION=X.Y.Z DRY_RUN=1`** is now actually a dry run.
  The guard's `exit 0` previously only exited its own subshell — Make
  kept going and ran the real `echo > VERSION` / `git commit` /
  `git tag` / `git push` lines anyway. The guard and the real-release
  sequence now share one `\`-joined shell block with `set -e`, so
  `DRY_RUN=1` stops cleanly before any side-effect runs. Closes TP-028.

No library, SPA, or CLI behaviour changed in this release.

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
