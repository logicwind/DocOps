---
title: Static HTML doc viewer for DocOps repositories
status: draft
coverage: required
date: "2026-04-24"
supersedes: []
related: [ADR-0014, ADR-0011, ADR-0012, ADR-0018]
tags: []
---

## Context

Agents and humans browsing a DocOps repository today see markdown files in a terminal or file tree. Raw markdown is functional but lacks the cross-referencing, navigation, and readability of rendered HTML — especially when reviewing the relationship graph (supersedes, related, requires edges) or comparing frontmatter across many documents.

ADR-0014 explicitly listed "No web UI" as a non-goal, calling it "Backlog.md / Linear's domain." That exclusion was aimed at hosted SaaS dashboards, issue trackers, and project-management web apps — heavy infrastructure that contradicts DocOps' substrate identity. A self-contained HTML viewer emitted by the CLI is a different category. It is closer to `godoc`, `rustdoc`, or `cargo doc`: a zero-dependency, locally rendered, offline-capable view of documentation that already exists in the repo.

The CLI already ships a read query layer (ADR-0018: `docops list`, `docops get`, `docops graph`). The HTML viewer is a presentation layer over the same data — not a new data surface, not a hosted service, not a multi-user dashboard.

ADR-0012's appendix selected Go as the implementation language and vetted `goldmark` for markdown AST. Go's `html/template` is stdlib. No new language or runtime is needed.

## Decision

Two new CLI subcommands serve a static HTML view of the current repository's DocOps documents:

```
docops html   — one-shot emitter; writes a directory of self-contained HTML files
docops serve  — localhost dev server with auto-rebuild on file change
```

### `docops html`

Emits a directory (default `docs/.html/`, configurable via `--output`) containing:

| File | Content |
|---|---|
| `index.html` | STATE overview + per-kind document listings with status badges and links |
| `ctx/{ID}.html` | Context detail — frontmatter table + rendered markdown body + reverse-edge links |
| `adr/{ID}.html` | ADR detail — frontmatter table + rendered markdown body + amendments section + reverse-edge links |
| `task/{ID}.html` | Task detail — frontmatter table + rendered markdown body + dependency/requires links |
| `state.html` | Full STATE.md rendered as HTML |

Flags:
- `--output <dir>` — output directory (default: `docs/.html`)
- `--base-url <url>` — rewrite relative links for hosting (e.g. GitHub Pages, custom domain)
- `--json` — emit `{ "files_written": N, "output_dir": "..." }`

### `docops serve`

Starts a localhost HTTP server that renders the same HTML from memory (no intermediate file writes), watches `docs/` recursively via `fsnotify`, and auto-rebuilds affected pages on any change.

Flags:
- `--port <int>` — port number (default: 8484)
- `--no-watch` — disable auto-rebuild; serve static output from a prior `docops html` run
- `--open` — open the default browser after startup
- `--json` — emit `{ "url": "http://localhost:...", "watch": true }`

Default URL: `http://localhost:8484/`

### Technology choices

| Component | Choice | Rationale |
|---|---|---|
| Markdown rendering | `github.com/yuin/goldmark` | Already vetted in ADR-0012 appendix. Zero-cgo, active maintenance. |
| HTML generation | Go `html/template` | Stdlib. Type-safe escaping. No build step. |
| CSS | Fully inlined per page | Zero external deps. Works offline. No CDN links. Embedded in Go source via `embed.FS`. |
| File watching | `github.com/fsnotify/fsnotify` | De-facto Go standard for filesystem events. Cross-platform. |
| GFM extensions | goldmark-extension tables, autolink, heading-id, strikethrough | Match the markdown conventions DocOps docs already use. |

### Design principles

1. **Self-contained.** Every HTML page works with zero network access. CSS, navigation, and content are in a single file. No JS framework, no build chain, no CDN.
2. **CLI-first.** The viewer is a CLI subcommand, not a separate application. Consistent with ADR-0011.
3. **Offline-capable.** Open a file in a browser on an airplane. No server required (after `docops html`).
4. **Read-layer consumer.** The viewer calls the same `internal/index` and `internal/loader` packages that power `docops list`/`get`/`graph`. It never reads `.index.json` directly — honoring ADR-0018.
5. **Hostable.** `--base-url` + static files means the output works on GitHub Pages, Netlify, S3, or any static host with zero configuration.

### Visual structure

Each page shares a common chrome:
- Top nav: Home / Context / Decisions / Tasks
- Breadcrumb: Home > ADR > ADR-0027
- Sidebar (detail pages): graph sidebar showing related/supersedes/requires links
- Footer: generated-by timestamp + docops version

Per-kind accent colors:
| Kind | Accent |
|---|---|
| CTX | Blue |
| ADR | Amber/Gold |
| TP | Green |

## Rationale

- **goldmark is already approved.** ADR-0012's appendix named it as the markdown AST library. Adding it to `go.mod` is a known quantity, not a new vetting event.
- **fsnotify is the only new dependency** beyond goldmark. It is cross-platform, well-maintained, and widely used in the Go ecosystem (Hugo, Air, etc.). Compiles to ~200 KB in the binary.
- **Self-contained HTML** matches the no-runtime-deps promise of ADR-0012. Users open a file in any browser without network or a running server.
- **Two commands** split the "I need static files to deploy" (`docops html`) from "I want live preview while editing" (`docops serve`). Neither requires the other.
- **Data via read layer** means the viewer automatically picks up every improvement to the CLI's query surface (new filters, new derived fields) without duplicating logic.

## Consequences

- **ADR-0014's "No web UI" clause is superseded.** The supersession is scoped: only the "web UI" non-goal is reversed. ADR-0014's other non-goals (no orchestration, no personas, no code generation, no execution harness, no automated PRs) remain intact.
- **Binary size** increases by goldmark (~500 KB compiled) + goldmark extensions (~100 KB) + fsnotify (~200 KB) + embedded CSS (~15 KB). Total delta under 1 MB — well within ADR-0012's 30 MB target.
- **Two new subcommands** are permanent CLI surface. They must be documented, tested, and maintained across schema changes.
- **CSS is embedded** in the compiled binary via Go's `embed.FS`. Theme changes require a recompile. This is acceptable for a standalone binary — users don't edit the binary's CSS.
- **No search UI** in v1. The viewer renders what `docops list`/`get` return. Full-text search in the browser is a future enhancement (could use client-side lunr.js or similar, but that adds JS — out of scope for v1).
- **`docs/.html/`** should be added to `.gitignore` by default. It is a generated artifact, like `docs/.index.json`, but not committed (it is too large and changes too frequently). The user may opt in to committing it for GitHub Pages deployment.
- **Amendments rendering** — if ADR-0025/TP-026 ships before the viewer, ADR detail pages render the amendments section. If not, the viewer ships without amendment support and gains it when TP-026 lands (the read layer handles it transparently).

## Rollout

1. This ADR lands `draft`.
2. **TP-030** — implement `docops html` (emitter + goldmark + templates + CSS).
3. **TP-031** — implement `docops serve` (HTTP server + fsnotify watch + live reload), depends on TP-030's rendering package.
4. **TP-032** — finalize CSS theme and embed it properly (depends on TP-030 for integration).
5. ADR promoted to `accepted` once TP-030 ships and the first `docops html` run produces valid output end-to-end.
