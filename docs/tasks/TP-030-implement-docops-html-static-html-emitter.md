---
title: Implement docops html — static SPA viewer emitter
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0027]
depends_on: []
---

## Goal

Ship `docops html` — a one-shot CLI subcommand that emits a self-contained, browsable viewer for the current DocOps repository. The viewer is a single HTML single-page app that reads the repository's graph + document bodies and renders them client-side with sidebar navigation, rendered markdown, cross-reference linkification, and a graph tab.

End state: running `docops html` in any DocOps-enabled repo writes `docs/.html/` containing one `index.html` SPA plus the data files it fetches, openable in any modern browser.

## Acceptance

### Output structure

`docops html --output docs/.html/` produces:

```
docs/.html/
  index.html          — the SPA (embedded in the Go binary via embed.FS)
  index.json          — copy of docs/.index.json, with raw-body paths baked in
  state.md            — copy of docs/STATE.md
  raw/
    context/CTX-*.md  — verbatim copies
    decisions/ADR-*.md
    tasks/TP-*.md
```

No per-doc HTML. No `ctx/`, `adr/`, `task/` rendered-page directories. Everything above the raw markdown is fetched and rendered by the SPA at runtime.

### The SPA (`index.html`)

A single HTML file, ~400 lines, embedded into the Go binary via `go:embed`. Pulls three libs from jsDelivr:

- `tailwindcss` play CDN — styling
- `marked` — markdown rendering
- `cytoscape` — graph tab

Sections:

- **Left sidebar**: grouped tree (CTX / ADR / TP), collapsible, status badge per row, search box, current-doc highlight.
- **Right pane**: breadcrumb → frontmatter table → rendered body. After rendering, a regex pass linkifies `ADR-\d+` / `CTX-\d+` / `TP-\d+` tokens in the HTML to `#/kind/ID`.
- **Graph tab**: Cytoscape instance consuming the same `index.json` edges. Click a node → routes to detail view.
- **Home view**: renders `state.md` + per-kind count tiles.
- **URL routing**: hash-based — `#/`, `#/state`, `#/graph`, `#/ctx/CTX-001`, `#/adr/ADR-0027`, `#/task/TP-030`.

The SPA lives at `internal/htmlviewer/index.html` and is embedded via `//go:embed index.html` in `internal/htmlviewer/spa.go`.

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
- `internal/htmlviewer/emit.go` — `Emit(idx *index.Index, root, outDir string) (int, error)` — writes `index.html`, `index.json`, `state.md`, and `raw/**`. Returns file count.
- `cmd/docops/cmd_html.go` — flag parsing, bootstrap, calls `htmlviewer.Emit`.

`cmd_html.go` calls `bootstrapIndex("html")` (shared helper — already used by `get`/`graph`/`list`/`next`), then `htmlviewer.Emit` to write the output directory. Follows the existing pattern of `cmd_index.go` for `--json` and `--output` handling.

### Data sourcing

- `index.json` is the serialized `internal/index.Index` produced by `internal/index.Build()` — same code path as `docops index`.
- `raw/*` files are copies of the source markdown files on disk. The SPA strips the YAML frontmatter before rendering with `marked`.
- `state.md` is read from `docs/STATE.md` if present; regenerated via `internal/state` if missing.

### Tests

- `internal/htmlviewer/emit_test.go`:
  - `Emit` against a temp repo fixture → assert `index.html`, `index.json`, `state.md`, `raw/decisions/ADR-0001.md` exist.
  - `index.json` parses as valid JSON and matches `index.Index` shape.
  - No external resource references written outside the output directory.
- `cmd/docops/cmd_html_test.go`:
  - `--json` mode emits valid `{"files_written":N,"output_dir":"..."}`.
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
