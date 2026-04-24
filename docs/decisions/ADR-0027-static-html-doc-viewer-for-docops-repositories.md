---
title: Static HTML doc viewer for DocOps repositories
status: accepted
coverage: required
date: "2026-04-24"
supersedes: []
related: [ADR-0014, ADR-0011, ADR-0012, ADR-0018]
tags: []
---

## Context

Agents and humans browsing a DocOps repository today see markdown files in a terminal or file tree. Raw markdown is functional but lacks the cross-referencing, navigation, and readability of rendered HTML — especially when tracing relationship edges (`supersedes`, `related`, `requires`, `depends_on`) or jumping from an inline `ADR-0014` reference in one doc to the doc itself.

ADR-0014 listed "No web UI" as a non-goal, aimed at hosted SaaS dashboards — heavy infrastructure that contradicts DocOps' substrate identity. A locally rendered viewer emitted by the CLI is a different category: closer to `godoc` / `cargo doc` — a zero-infrastructure view of documentation that already exists in the repo.

The CLI already ships a read query layer (ADR-0018: `docops list`, `docops get`, `docops graph`) and an enriched graph JSON (`docs/.index.json`). The viewer is a presentation layer over the same data — not a new data surface, not a hosted service, not a multi-user dashboard.

An earlier draft of this ADR specified Go-side rendering via `goldmark` with per-document HTML files emitted by the CLI, fully inlined CSS, and no JavaScript. Before any implementation landed we pivoted (see "Amendment 2026-04-24" below). The sections below describe the shipping design.

## Decision

Two new CLI subcommands expose a **single-page web viewer** for the current repository:

```
docops html    — emit a self-contained viewer directory (SPA + data)
docops serve   — localhost HTTP server that serves the same viewer
```

The viewer is **one HTML file**. All rendering, navigation, and graph layout happen client-side. Go's job is to emit data and serve files.

### Architecture

```
┌─────────────────────────────────────────────┐
│  Go CLI (docops)                            │
│    - builds a viewer bundle (index + every  │
│      doc body + STATE.md, all in one JSON)  │
│    - copies embedded index.html             │
└─────────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────┐
│  Browser (one index.html, ~530 lines)       │
│    - fetches the bundle once                │
│    - sidebar: grouped tree (CTX / ADR / TP) │
│    - right pane: rendered markdown body     │
│    - graph tab: pinned-column Cytoscape     │
│    - hover/click highlight, dbl-click opens │
│    - hash routing (#/ADR/ADR-0027)          │
└─────────────────────────────────────────────┘
```

### What Go emits

`docops html --output docs/.html/` writes **just two files**:

| File | Purpose |
|---|---|
| `index.html` | The SPA. Embedded in the Go binary via `embed.FS` at compile time; written verbatim. |
| `index.json` | Viewer bundle: the enriched index graph plus every CTX/ADR/TP body (frontmatter stripped) and STATE.md inlined at the top level. One fetch, zero follow-up round-trips. |

No per-doc HTML. No template fan-out. No CSS asset pipeline. No raw/ copy of source markdown. No separate `state.md` file.

`docops serve` serves the same two artifacts from memory — `/` returns the embedded SPA bytes, `/index.json` rebuilds the bundle on each request via `internal/index.Build()`. No fsnotify, no live reload — the browser reload refreshes everything, sufficient for a dev viewer and avoids SSE + debouncing complexity.

### The SPA

One HTML file with three external `<script>`/`<link>` tags, all from jsDelivr:

| Library | Purpose | Approx. size gzipped |
|---|---|---|
| `tailwindcss` (play CDN) | Styling. Zero build step. | ~50 KB |
| `marked` | Markdown → HTML. | ~30 KB |
| `cytoscape` | Graph layout and interaction. | ~120 KB |

Total first-load cost: well under 250 KB gzipped, browser-cached after first visit. Compared to the Go-side plan (goldmark + fsnotify + embedded CSS, ~1 MB of extra binary), this trades a one-time network fetch for a smaller Go binary and a far better interactive graph.

Structure:

- **Left sidebar** — reads the bundle, groups by kind (CTX / ADR / TP), collapsible sections, search box, status badges. Click to load into right pane. Current doc highlighted.
- **Right pane**:
  - Breadcrumb (Home > Decision > ADR-0027).
  - Frontmatter table — every field rendered as key/value; arrays of IDs linkified to other docs.
  - Rendered markdown body via `marked`, read straight from `doc.body` in the bundle (no extra fetch).
  - After rendering, a regex pass linkifies every bare `ADR-\d+`, `CTX-\d+`, `TP-\d+` token in the body to its detail view.
  - Reverse-edge chips (Referenced by, Superseded by, Derived ADRs, Active tasks, Blocks) linked back into the SPA.
- **Graph tab** — Cytoscape.js with a pinned column layout: **CTX on the left (1 column), ADR in the middle (2 columns), TP on the right (3 columns)**. IDs are column-major so they stack in numerical order down each column. Section headers sit above each group; a legend in the top-right corner shows kind + edge-type colors. Edge colors by type: `supersedes` red, `requires` blue, `depends_on` purple, `related` gray.
- **Graph interactions** — hover a node to focus it and its neighborhood (everything else fades); single tap pins that selection; tap on blank background clears; double tap opens the doc's detail view. Neighbor set = outgoing + incoming edges + the other endpoint of each. Section-header nodes are non-interactive.
- **URL routing** — `#/`, `#/state`, `#/CTX/CTX-001`, `#/ADR/ADR-0027`, `#/TP/TP-030`, `#/graph`. Deep-links from terminal output / chat work.
- **Home view** — STATE.md (from the bundle's inlined `state_md`) + per-kind count tiles.

### Command flags

`docops html`:

| Flag | Default | Description |
|---|---|---|
| `--output` / `-o` | `docs/.html` | Output directory (created if absent; contents replaced if present). |
| `--base-url` | (empty) | Path prefix for all data fetches (for hosting behind a subpath). |
| `--json` | off | Emit `{ "files_written": N, "output_dir": "..." }` to stdout. |

`docops serve`:

| Flag | Default | Description |
|---|---|---|
| `--port` / `-p` | `8484` | Port to listen on. |
| `--open` | off | Open the default browser after startup. |
| `--json` | off | Emit `{ "url": "http://localhost:...", "port": N }`. |

### Design principles

1. **Light on the Go side.** Go ships data and one HTML file. No markdown rendering, no templating, no file watching, no SSE.
2. **CLI-first.** Still two subcommands — consistent with ADR-0011.
3. **Single HTML file.** The SPA is one file, human-readable, ~400 lines. Edit it in any editor. No bundler.
4. **CDN for libs.** Tailwind / `marked` / `cytoscape` come from jsDelivr. Modern browsers cache them across sites. Vendoring locally is one config flag away if needed later.
5. **Read-layer consumer.** `docops html` and `docops serve` both call `internal/index.Build()` — the same code path as `docops index` / `docops get` / `docops graph`. The viewer never reads `docs/.index.json` directly from Go (honors ADR-0018). The browser *does* fetch `index.json`, which is the emitted JSON artifact — a deliberate, documented surface.

## Rationale

- **Client-side rendering avoids goldmark entirely.** `marked` is a single JS file, ~30 KB gzipped, handles GFM, fast, well-maintained. No Go dependency, no binary bloat, theme tweaks are edits to one HTML file instead of recompiles.
- **Cytoscape.js gives a real graph for free.** Zoom, pan, layout engine, click-to-navigate. Doing the equivalent in Go-rendered static HTML would require SVG generation and hand-rolled layout.
- **One HTML file is maintainable.** No build system, no framework lock-in, no transpilation. Any future maintainer can read it top-to-bottom.
- **Tailwind via CDN is zero-config.** The play CDN compiles on the fly from class names in the HTML. No `postcss`, no `npm`, no `tailwind.config.js`. Acceptable for a dev viewer; we can switch to a static `tailwind.css` build artifact later if we ever need pure-offline.
- **No live reload in v1.** The editor-save → browser-refresh loop is a manual refresh. This cuts fsnotify + SSE + debouncing + reconnect logic entirely. If users ask for it, it's a ~30-line addition later.

## Consequences

- **ADR-0014's "No web UI" clause is superseded.** Scoped: only the "web UI" non-goal is reversed. ADR-0014's other non-goals (no orchestration, no personas, no code generation, no execution harness, no automated PRs) remain intact.
- **First load needs network access** to fetch Tailwind / marked / Cytoscape from jsDelivr. After that, the browser cache handles it. For truly offline use, users can either (a) pre-warm the browser cache, or (b) wait for a future `--vendor` flag that inlines the libs. This is a conscious tradeoff for binary size and render quality.
- **Binary delta is tiny** — one embedded HTML file (~15 KB) plus ~60 lines of Go for each of `cmd_html` and `cmd_serve`. No new Go dependencies.
- **Two new subcommands** are permanent CLI surface. They must be documented, tested, and maintained across schema changes — but the maintenance surface is the SPA, not template packages.
- **`docs/.html/`** should be added to `.gitignore` by default. Users may opt in to committing it for GitHub Pages deployment (the output directory is a working static site).
- **Amendments rendering** is transparent — `internal/index.IndexedDoc` already has an `Amendments` field (once TP-026 lands); the SPA renders it if present, skips it if absent. No ADR coupling.

## Amendment 2026-04-24 — shifted to client-side SPA

The original draft of this ADR specified Go-side rendering with `goldmark`, per-doc HTML files, fully inlined CSS, and no JavaScript. Before any code landed, we reconsidered:

- The sidebar / detail-pane UX the user wanted needs interactive navigation (search, collapse, auto-linkify, current-doc highlight). Server-rendered static HTML makes this awkward; JS makes it natural.
- The graph view was the strongest motivator. A real force-directed graph with click-to-navigate is trivial with Cytoscape.js and ~100 lines of Go to hand-draw an SVG fallback.
- The Go binary stays slim and the SPA stays editable as a plain file.

The decision (ship a CLI-launched HTML viewer) is unchanged; the implementation strategy pivoted before any code was written. The "Rationale" and "Consequences" sections above reflect the SPA direction. This amendment is noted inline (per ADR-0025 convention) because (a) the ADR is still `draft`, so no immutable contract was broken, and (b) the pivot is worth recording so future readers don't wonder why the ADR lost its goldmark references.

## Amendment 2026-04-24 (later) — bundled JSON + pinned graph + interactions

Three refinements that landed during implementation and are worth capturing here so the ADR matches shipped reality:

1. **Bundled JSON.** The initial SPA design fetched `raw/<path>` per doc-click and a separate `/state.md`. In practice that meant copying 60+ markdown files into `docs/.html/raw/` and making two extra round-trips per navigation. Shipped design inlines every doc body and STATE.md into the emitted `index.json`. Output is now two files (`index.html` + `index.json`), navigation is instant, `/raw/*` and `/state.md` routes are gone.
2. **Pinned column graph layout.** Cytoscape's default force-directed layout was hard to read for a DocOps graph — strongly clustered but no obvious grouping. The shipped layout is `preset` with computed positions: CTX in 1 column on the left, ADR in 2 columns in the middle, TP in 3 columns on the right; column-major fill so IDs stack in numerical order down each column. Section headers above each group; legend with hover/click/dbl-click hints in the top-right corner.
3. **Focus-on-hover interactions.** Clicking a node originally navigated to its detail view, which was noisy when what the user wanted was "just show me what this connects to." Shipped model: hover focuses a node's closed neighborhood (faded everything else), single tap pins that focus (click again or tap blank to clear), double-tap navigates. This mirrors Kibana / Linkurious / OSINT graph-viewer conventions.

No decision change — these are implementation details that fell out of using the viewer against this repo's own docs. Captured here (rather than a superseding ADR) because the direction is unchanged and the ADR is still `draft`.

## Rollout

1. This ADR lands `draft`.
2. **TP-030** — implement `docops html` (SPA copy + index.json + state.md + raw bodies).
3. **TP-031** — implement `docops serve` (localhost HTTP server over the same data), depends on TP-030's `internal/htmlviewer` package.
4. **TP-032** is obsolete under the SPA approach (no Go-side CSS theme to design) — closed out via TP-030.
5. ADR promoted to `accepted` once TP-030 ships and the first `docops html` run produces a viewer that loads against this repo's documents.
