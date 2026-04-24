---
title: Implement docops serve — live dev server with auto-rebuild
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0027]
depends_on: [TP-030]
---

## Goal

Ship `docops serve` — an HTTP server that renders the same HTML as `docops html` from memory, watches `docs/` for changes, and auto-rebuilds affected pages so the browser stays in sync during editing.

End state: `docops serve` starts, opens no files to disk, renders on request, detects edits under `docs/`, and pushes updates to the browser without a manual refresh.

## Acceptance

### Server behavior

- Listens on `localhost:{port}` (default `8484`).
- Serves the same HTML content as `docops html` but renders on-demand from memory — no `docs/.html/` directory is created.
- Routes:
  - `GET /` → index.html (STATE overview + listings)
  - `GET /state` → state.html
  - `GET /ctx/{ID}` → context detail page
  - `GET /adr/{ID}` → ADR detail page
  - `GET /task/{ID}` → task detail page
  - `GET /health` → `{ "status": "ok" }` (for CI checks)
- Responds with `Content-Type: text/html; charset=utf-8`.
- Returns 404 for unknown document IDs with a styled "Not found" page.
- Returns 500 for rendering errors with the error message.

### Auto-rebuild

- Uses `github.com/fsnotify/fsnotify` to watch `docs/` recursively (excluding `docs/.docops/`, `docs/.html/`, and `docs/.index.json`).
- On any file change event (create, write, rename, remove under `docs/context/`, `docs/decisions/`, `docs/tasks/`):
  1. Rebuild the in-memory index via `internal/index.BuildIndex()`.
  2. Invalidate cached rendered pages for the changed document + its reverse-edge neighbors.
  3. Push an SSE event (`event: reload`) to connected browsers (via `EventSource` JS snippet injected into each page's `<head>`).
- Debounce: coalesce events within 200ms to avoid rebuild storms from editor save-and-checkin sequences.
- If index rebuild fails (e.g. invalid frontmatter), serve the last-good rendered pages and log the error. Do not crash.

### Live-reload mechanism

- Each HTML page includes a small inline `<script>` in the `<head>` that opens an `EventSource` connection to `/events` on the same host.
- Server side: `/events` endpoint holds open connections and pushes `event: reload` when the index is rebuilt.
- Client side: on receiving `reload` event, call `location.reload()`.
- If the EventSource connection drops, the script reconnects after 1 second with exponential backoff (max 8s).
- **No build step, no framework, no CDN.** The script is ~20 lines of vanilla JS inlined in the template.

### Command flags

| Flag | Default | Description |
|---|---|---|
| `--port` / `-p` | `8484` | Port to listen on |
| `--no-watch` | off | Disable fsnotify watcher; serve static output from a prior `docops html --output` instead |
| `--open` | off | Open the system browser after startup |
| `--json` | off | Emit `{ "url": "http://localhost:...", "watch": true }` to stdout, suppress human output |

Exit: 0 on clean shutdown (SIGINT/SIGTERM), 1 on fatal start error (port in use, no docs directory).

### Go implementation

- New command file: `cmd/docops/cmd_serve.go`
- New package: `internal/htmlviewer/server.go` (or split into `internal/httpserver/` if preferred)
- `server.go` contains:
  - `Server` struct with `http.Handler`, index cache, template cache, fsnotify watcher.
  - `Start(ctx context.Context) error` — starts listening.
  - `Watch(ctx context.Context) error` — starts fsnotify loop.
  - `RenderPage(id string) ([]byte, error)` — renders one page from current index.
- `cmd_serve.go` wires flags, creates `Server`, starts HTTP + watcher goroutines, blocks on signal handling.
- Graceful shutdown: `http.Server.Shutdown(ctx)` with 5s timeout, `watcher.Close()`.

### Sourcing from TP-030's rendering package

- `internal/htmlviewer/renderer.go` (from TP-030) exports a `RenderIndex`, `RenderDetail`, `RenderState` function that takes an `index.IndexedDoc` slice and returns `[]byte` of HTML.
- `internal/htmlviewer/server.go` calls these same functions, caching results per page ID.
- No duplication of template logic between `cmd_html` and `cmd_serve`.

### Tests

- Unit tests for:
  - SSE event format.
  - Route matching (valid IDs, 404 for unknown).
  - Debounce logic (events within 200ms coalesce).
- Integration tests:
  - Start server on random port, `GET /` returns 200 with valid HTML.
  - `GET /adr/ADR-0001` returns 200 with frontmatter content.
  - `GET /adr/ADR-9999` returns 404.
  - Write a new file to `docs/context/`, assert index rebuild + next request reflects it.
  - Send SIGINT, assert graceful shutdown within 5s.
- Negative tests:
  - Port already in use → clear error message.
  - Running in non-DocOps repo → exit 1 with "no docs/ directory found".

## Notes

- The inline JS for live reload is the only JavaScript in the entire viewer. It is intentionally minimal and inlined to preserve the self-contained, zero-external-deps philosophy. The JS does not render any content — it only triggers a browser reload.
- `--no-watch` is useful for serving pre-built output from `docops html` in environments where filesystem watching is unavailable (Docker with mounted volumes, some CI sandboxes). In this mode, the server just serves static files from the `--output` directory using `http.FileServer`.
- Consider adding a `--poll-interval` flag as a fallback when `fsnotify` does not work on certain filesystems (NFS, some FUSE mounts). For v1, document the limitation instead.
- The `--open` flag uses `exec.Command("open", url)` on macOS, `xdg-open` on Linux, `start` on Windows. Handle "no browser available" gracefully (log a message, don't crash).

## Out of scope

- HTTPS / TLS (dev server is localhost only).
- Authentication.
- Websocket-based live reload (SSE is sufficient and simpler).
- Multi-repo aggregation.
- Custom routes or plugin system.
