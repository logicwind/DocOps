# Slice 5: Web Console — Plan

> Status: Fleet, Chat, Events, Editor, and Memory views all shipped.
> Editor (M4) loads via `GET /v1/fleet/export`, previews via
> `POST /v1/apply?preview`, applies with confirm. Memory route added in
> commit 3623b37 — agent picker, scope tabs (agent / thread:\* / fleet
> read-only), markdown editor, facts table with FTS search and prune.

---

## Setup

```bash
pnpm dlx shadcn@latest init --name console --preset base-nova --template vite
```

- `packages/console/` as a new pnpm workspace package
- React + Tailwind + shadcn/ui components
- Vite for dev/build
- Bun backend serves Vite output + proxies `/v1/*` to zundd `:4000`
- Console runs on `:3000`
- Does NOT replace the dev harness (`:9999`) — harness stays as testing/debugging ground

## Layout: Top Nav + Contextual Sidebars

```
+=====[Zund]=====[Fleet]==[Chat]==[Editor]==[Events]============[theme][?]=+
|                                                                           |
|  +--context-sidebar--+--main-panel-----------------------------------+   |
|  |                   |                                               |   |
|  | (varies by       |  (varies by section)                          |   |
|  |  section)        |                                               |   |
|  |                  |                                               |   |
|  +------------------+-----------------------------------------------+   |
+==========================================================================+
```

Top nav for section switching. Each section has its own contextual sidebar.

## Views

### 1. Fleet (`/fleet`)

- Agent cards grid — name, status dot, model, uptime, health
- Click card to expand or navigate to detail
- Sidebar: filter/search (later)
- Data: `GET /v1/fleet/status`

### 2. Chat (`/chat`) — the power view

```
sidebar:                    main:
+-------------------+      +------------------------------------------+
| > Agents          |      | [agent-name]  * ready  model/id    [+new]|
|   * writer        |      |------------------------------------------|
|   * reviewer      |      |                                          |
|   o analytics     |      |  message bubbles, streaming,             |
|                   |      |  tool calls, markdown                    |
| > Sessions        |      |                                          |
|   today 2:30pm    |      |                                          |
|   today 11:15am   |      |                                          |
|   yesterday       |      |                                          |
|                   |      |------------------------------------------|
|                   |      | [textarea input]                    [>]  |
+-------------------+      +------------------------------------------+
```

- Sidebar has two collapsible sections: **Agents** (with status dots) and **Sessions** (per agent)
- Sessions section shows "No session history yet" until session API exists
- Chat panel: streaming SSE, markdown rendering, tool call blocks
- Data: `POST /v1/agents/:name/message`, `GET /v1/agents/:name/state`

### 3. Editor (`/editor`)

- Split pane: YAML editor (left) + preview diff (right)
- Bottom bar: [Load Fleet] [Preview] [Apply]
- YAML editor: CodeMirror 6
- Load current fleet via `GET /v1/fleet/export`
- Preview via `POST /v1/apply { preview: true }`
- Apply via `POST /v1/apply`
- Lower priority than Chat + Fleet — can be a fast follow

### 4. Events (`/events`)

- Live scrolling event log from `GET /v1/events` (SSE)
- Each event: timestamp + type + payload
- Sidebar: filter checkboxes (agent._, fleet._, health.\*)

## shadcn Components

- `sidebar` — contextual per-section sidebars
- `card` — agent cards on fleet page
- `badge` — status badges (running/stopped/degraded)
- `button`, `input`, `textarea` — basics
- `tabs` — within pages
- `dialog` — confirmations (apply, delete)
- `toast` / `sonner` — notifications
- `scroll-area` — chat message list
- `separator`, `skeleton` — polish

## Routing

React Router, client-side. Bun backend serves `index.html` for all non-`/v1/*` routes.

## Data Layer

- Plain `fetch` + React hooks (`useState`, `useEffect`) to start
- Custom `useEventSource` hook for SSE (`GET /v1/events`)
- TanStack Query later when caching/refetching complexity warrants it

## Backend (`packages/console/server.ts`)

Minimal Bun.serve:

- Serve Vite build output (static files)
- Proxy `/v1/*` to zundd `:4000`
- Health check at `/health`
- In dev: Vite dev server handles HMR, proxy still works

## Session History Strategy

Pi stores sessions as JSONL files in `~/.zund/data/sessions/{agent}/`.
Currently no API to list/load them (POC 11 not started).

Plan: design the chat UI to accept a `messages[]` prop. When the session API
lands (`GET /v1/agents/:name/sessions`), it's a data source swap, not a rewrite.

## Not in Slice 5 (future)

- Auth (login, JWT, user management)
- Team/workflow views
- Agent creation wizard (visual alternative to YAML)
- Monitoring/metrics dashboards
- Settings page

## Reference

- Experiment 10B (`experiments/10b-custom-web-ui/`) — proven chat UI patterns: SSE streaming, markdown renderer, tool call blocks, agent sidebar, dark/light theme
- Dev harness (`test/harness/`) — Preact+HTM reference for fleet management UI
- zundd API (`packages/daemon/src/api/server.ts`) — all endpoints already exist
