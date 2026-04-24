---
title: Implement docops serve — localhost viewer server
status: done
priority: p2
assignee: unassigned
requires: [ADR-0027]
depends_on: [TP-030]
---

## Shipped — v0.5.0 (2026-04-24)

Implementation lives in `internal/htmlviewer/serve.go` +
`cmd/docops/cmd_serve.go`. `docops serve [--port N] [--open] [--json]`
listens on `127.0.0.1:8484` by default, serves the embedded SPA plus
a freshly-rebuilt `/index.json` on every request, and exits cleanly on
SIGINT/SIGTERM.

## Goal

Ship `docops serve` — a localhost HTTP server that serves the same SPA + data as `docops html`, but rebuilds the index on each page load instead of writing files to disk. Stays in memory, no fsnotify, no live reload in v1.

End state: `docops serve` starts on `localhost:8484` (default), serves `index.html` from the embedded SPA, serves `index.json` freshly built via `internal/index.Build()` on every request, and serves raw markdown from disk. Press Ctrl-C to stop.

## Acceptance

### Server behavior

- Listens on `localhost:{port}` (default `8484`).
- Routes:
  - `GET /` and `GET /index.html` → embedded SPA bytes (`internal/htmlviewer.SPA`).
  - `GET /index.json`             → fresh viewer bundle via `internal/htmlviewer.BuildBundle` (enriched index + every doc body + STATE.md, all in one payload).
  - `GET /health`                 → `{"status":"ok"}` for CI / scripted checks.
- `Content-Type` set appropriately: `text/html`, `application/json`.
- Unknown paths → 404.

No `/raw/*` or `/state.md` route — everything the SPA renders is in the single bundle.

### Command flags

| Flag | Default | Description |
|---|---|---|
| `--port` / `-p` | `8484` | Port to listen on |
| `--open` | off | Open default browser on startup (`open` / `xdg-open` / `start`) |
| `--json` | off | Emit `{"url":"http://localhost:...", "port":N}` to stdout and suppress the human banner |

Exit: `0` on SIGINT / SIGTERM shutdown, `1` on fatal start error (port in use, no docs dir).

### Go implementation

- `cmd/docops/cmd_serve.go` — flag parsing, `http.Server` setup, signal handling.
- `internal/htmlviewer/serve.go` — `Handler(root string, cfg config.Config) http.Handler` returning a `*http.ServeMux` that wires the routes above. Rebuilds the bundle (index + bodies + state) on every `/index.json` request.
- Graceful shutdown: `http.Server.Shutdown(ctx)` with a 5 s deadline on SIGINT.

### No live reload in v1

Browser reload (Cmd-R) is sufficient — it re-fetches the bundle and everything updates. No fsnotify, no SSE, no debouncing. If live reload becomes a common ask, we add a `/events` SSE endpoint + a ~20-line JS snippet in a follow-up task — explicitly deferred here to keep v1 tight.

### Tests

- `internal/htmlviewer/serve_test.go`:
  - `GET /` → 200 + `text/html` + contains `<title>DocOps`.
  - `GET /index.json` → 200 + parses as `Bundle` (with `state_md` + per-doc `body`).
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
