---
title: Implement docops serve — localhost viewer server
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0027]
depends_on: [TP-030]
---

## Goal

Ship `docops serve` — a localhost HTTP server that serves the same SPA + data as `docops html`, but rebuilds the index on each page load instead of writing files to disk. Stays in memory, no fsnotify, no live reload in v1.

End state: `docops serve` starts on `localhost:8484` (default), serves `index.html` from the embedded SPA, serves `index.json` freshly built via `internal/index.Build()` on every request, and serves raw markdown from disk. Press Ctrl-C to stop.

## Acceptance

### Server behavior

- Listens on `localhost:{port}` (default `8484`).
- Routes:
  - `GET /`                     → embedded SPA bytes (`internal/htmlviewer.SPA`).
  - `GET /index.json`           → fresh `internal/index.Build()` output, JSON-encoded.
  - `GET /state.md`             → `docs/STATE.md` from disk (regenerated via `internal/state` if stale).
  - `GET /raw/{path}`           → file on disk under `docs/context|decisions|tasks/...` (path-traversal-safe).
  - `GET /health`               → `{"status":"ok"}` for CI / scripted checks.
- `Content-Type` set appropriately: `text/html`, `application/json`, `text/markdown`.
- Unknown paths → 404 with a small inline "Not found" HTML snippet.

### Command flags

| Flag | Default | Description |
|---|---|---|
| `--port` / `-p` | `8484` | Port to listen on |
| `--open` | off | Open default browser on startup (`open` / `xdg-open` / `start`) |
| `--json` | off | Emit `{"url":"http://localhost:...", "port":N}` to stdout and suppress the human banner |

Exit: `0` on SIGINT / SIGTERM shutdown, `1` on fatal start error (port in use, no docs dir).

### Go implementation

- `cmd/docops/cmd_serve.go` — flag parsing, `http.Server` setup, signal handling.
- `internal/htmlviewer/serve.go` — `Handler(root string, cfg *config.Config) http.Handler` returning a `*http.ServeMux` that wires the routes above. Rebuilds the index on `/index.json` requests.
- Path-traversal guard on `/raw/` — reject anything containing `..` or resolving outside `docs/`.
- Graceful shutdown: `http.Server.Shutdown(ctx)` with a 5 s deadline on SIGINT.

### No live reload in v1

The SPA already triggers a full data refresh on navigation (hash change → re-fetch `index.json` / raw body). Editors that save a file and switch to the browser can hit Cmd-R. If live reload becomes a common ask, we add a `/events` SSE endpoint + a ~20-line JS snippet in a follow-up task — explicitly deferred here to keep v1 tight.

### Tests

- `internal/htmlviewer/serve_test.go`:
  - `GET /` → 200 + `text/html` + contains `<title>DocOps`.
  - `GET /index.json` → 200 + parses as `index.Index`.
  - `GET /raw/decisions/ADR-0001.md` against a fixture → 200 + matches file bytes.
  - `GET /raw/../../etc/passwd` → 400.
  - `GET /nope` → 404.
- `cmd/docops/cmd_serve_test.go`:
  - Smoke test: start server on random port (`--port 0`), hit `/health`, shut down cleanly.

## Notes

- `--open` is best-effort: log a message if the OS open command fails rather than crashing.
- `--port 0` should ask the OS for a free port; emit the actual bound address in the banner / JSON output.
- The server is for the current user on the current machine. No auth, no TLS, no remote binding.

## Out of scope

- Auto-rebuild on file change (fsnotify, SSE).
- HTTPS / TLS.
- Authentication or rate limiting.
- Multi-repo aggregation.
- Custom routes or plugins.
