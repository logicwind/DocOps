---
title: Implement docops html — static SPA viewer emitter
status: done
priority: p2
assignee: unassigned
requires: [ADR-0027]
depends_on: []
---

## Shipped — v0.5.0 (2026-04-24)

Implementation lives in `internal/htmlviewer/` (`spa.go`, `bundle.go`,
`emit.go`, `index.html`) and `cmd/docops/cmd_html.go`. `docops html
[--output PATH] [--base-url URL] [--json]` emits exactly two files
(`index.html` + `index.json`) into the output directory.

## Goal

Ship `docops html` — a one-shot CLI subcommand that emits a self-contained, browsable viewer for the current DocOps repository. The viewer is a single HTML single-page app that reads the repository's graph + every document body + STATE.md out of a single bundled JSON and renders everything client-side with sidebar navigation, rendered markdown, cross-reference linkification, and a pinned-column graph tab.

End state: running `docops html` in any DocOps-enabled repo writes exactly **two files** into `docs/.html/` (`index.html` + `index.json`), openable in any modern browser.

## Acceptance

### Output structure

`docops html --output docs/.html/` produces:

```
docs/.html/
  index.html          — the SPA (embedded in the Go binary via embed.FS)
  index.json          — viewer bundle: enriched index + doc bodies + STATE.md
```

Two files, always. No per-doc HTML, no `raw/` copy directory, no separate `state.md`. Navigation is instant after first load because every body is already in the bundle.

The bundle shape:

```
{
  "generated_at": "...",
  "version": 1,
  "state_md": "<contents of docs/STATE.md, or empty>",
  "docs": [
    { ...IndexedDoc fields..., "body": "<markdown with frontmatter stripped>" },
    ...
  ]
}
```

### The SPA (`index.html`)

A single HTML file, ~530 lines, embedded into the Go binary via `go:embed`. Pulls three libs from jsDelivr:

- `tailwindcss` play CDN — styling
- `marked` — markdown rendering
- `cytoscape` — graph tab

Sections:

- **Left sidebar**: grouped tree (CTX / ADR / TP), collapsible, status badge per row, search box, current-doc highlight.
- **Right pane**: breadcrumb → frontmatter table → reverse-edge chips → rendered body from `doc.body` in the bundle. After rendering, a regex pass linkifies `ADR-\d+` / `CTX-\d+` / `TP-\d+` tokens in the HTML to `#/KIND/ID`.
- **Graph tab**: Cytoscape instance with a **pinned column layout** — CTX in 1 column (left), ADR in 2 columns (middle), TP in 3 columns (right); column-major fill so IDs stack in numerical order. Section-header label nodes above each group; a legend with hover/click/dbl-click hints in the top-right. Edge colors by type: `supersedes` red, `requires` blue, `depends_on` purple, `related` gray.
- **Graph interactions**: hover a node → focus its closed neighborhood, fade everything else. Single tap pins that focus; tap blank background clears. Double-tap opens the doc's detail view.
- **Home view**: renders the bundle's inlined `state_md` + per-kind count tiles.
- **URL routing**: hash-based — `#/`, `#/state`, `#/graph`, `#/CTX/CTX-001`, `#/ADR/ADR-0027`, `#/TP/TP-030`.

The SPA lives at `internal/htmlviewer/index.html` and is embedded via `//go:embed index.html` in `internal/htmlviewer/spa.go`. The cytoscape instance is exposed on `window.__docopsCy` as an escape hatch for devtools and end-to-end tests.

### Command flags

| Flag | Default | Description |
|---|---|---|
| `--output` / `-o` | `docs/.html` | Output directory (created if absent; contents replaced if present) |
| `--base-url` | (empty) | Path prefix rewritten into the SPA's fetch calls, for hosting behind a subpath |
| `--json` | off | Emit `{ "files_written": N, "output_dir": "..." }` to stdout |

Exit codes: `0` success, `2` invalid flags / bootstrap error, `1` runtime error (write failure).

### Go implementation

New files:

- `internal/htmlviewer/spa.go` — exports `SPA []byte` via `//go:embed index.html`.
- `internal/htmlviewer/index.html` — the SPA source.
- `internal/htmlviewer/bundle.go` — `BuildBundle(idx, cfg, root) (*Bundle, error)` — reads STATE.md + each doc body off disk (with frontmatter stripped) and returns the combined JSON payload. Shared by emit and serve.
- `internal/htmlviewer/emit.go` — `Emit(idx, cfg, root, opts) (int, error)` — writes `index.html` and `index.json` (from `BuildBundle`). Returns file count (2 on success). Also handles `--base-url` by injecting `<base href>` into the SPA head.
- `cmd/docops/cmd_html.go` — flag parsing, bootstrap, calls `htmlviewer.Emit`.

`cmd_html.go` calls `bootstrapIndex("html")` (shared helper — already used by `get`/`graph`/`list`/`next`), then `htmlviewer.Emit` to write the output directory.

### Data sourcing

- Bundle is produced by `BuildBundle` from the in-memory `*index.Index` (no `.index.json` file read).
- Each `BundleDoc` embeds `index.IndexedDoc` and adds a `Body string` field. Body is the source `.md` file with its leading `---\n...\n---\n` block stripped.
- `state_md` at the top level of the bundle is the verbatim contents of `cfg.Paths.State` (default `docs/STATE.md`), empty string if absent.

### Tests

- `internal/htmlviewer/bundle_test.go`:
  - `BuildBundle` against a fixture → asserts `state_md` populated, every doc has a non-empty body, frontmatter is stripped (body does not start with `---`).
- `internal/htmlviewer/emit_test.go`:
  - `Emit` against a temp repo fixture → exactly two files written (`index.html`, `index.json`); no `raw/` dir created.
  - `index.json` parses as `Bundle` with the expected shape.
- `cmd/docops/cmd_html_test.go`:
  - `--json` mode emits valid `{"files_written":2,"output_dir":"..."}`.
  - Non-DocOps repo (no `docops.yaml`) → exit 2 with clear error.
  - `--output` on a parent dir that doesn't exist → creates it.

## Notes

- Output directory `docs/.html/` is appended to `.gitignore` by `docops upgrade` (separate follow-up if not already covered).
- `--base-url` rewrites the `<base href>` tag in the emitted `index.html` only. Data fetches use relative paths below `<base>`.
- Amendments field on ADRs renders automatically if present in the index — the SPA walks frontmatter keys, no ADR coupling.
- No fsnotify, no SSE, no live reload here — that's `docops serve`'s domain (TP-031), and even there it's not shipping in v1.

## Out of scope

- Live reload / file watching (see TP-031 for the dev server; live reload deferred).
- Vendoring Tailwind / marked / cytoscape locally for offline (future `--vendor` flag).
- Syntax highlighting in code blocks (marked default styling is sufficient for v1).
- Authentication, multi-repo aggregation, PDF export.
