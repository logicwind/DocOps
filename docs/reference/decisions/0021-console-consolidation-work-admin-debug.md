---
id: "0021"
title: "Console consolidation — Work / Admin / Debug IA, harness removed"
date: 2026-04-16
status: accepted
implementation: done
supersedes: []
superseded_by: null
related: ["0014"]
tags: [console, ui, ia, dx, harness-removal]
---

# 0021 · Console consolidation — Work / Admin / Debug IA, harness removed

Date: 2026-04-16
Status: accepted
Related: ADR 0014 (user-agent message endpoint + SSE)

## Context

The repo carried two parallel web UIs:

- `apps/console/` — React + Vite + Tailwind console (Chat, Fleet,
  Events, Memory, Secrets, Editor) wired against zundd via `apps/console/server.ts`.
- `test/harness/` — Preact + htm dev dashboard (Agents, Fleet, Memory,
  Secrets, Tests) served from `test/harness/server.ts`.

They had drifted into ~70% overlap on fleet, memory, and secrets, with
each surface accumulating unique features the other lacked (console:
fact pruning, stale-secret banners; harness: SOPS vault badge, sample
YAML picker, 11-row API test grid, per-agent Stop/Restart). Keeping both
meant double maintenance and forking of truth.

The user also flagged that the old flat nav (Fleet · Chat · Events ·
Memory · Secrets · Editor) mixes end-user, operator, and developer
concerns on one row. End users don't need Secrets or Editor.

## Decision

Collapse both surfaces into `apps/console/` under a three-group
information architecture:

- **Work** — `/chat`, `/memory` (future: tasks, approvals, knowledge,
  assets). Daily end-user surface.
- **Admin** — `/admin/fleet`, `/admin/editor`, `/admin/secrets`,
  `/admin/events`. DevOps-style operator surface.
- **Debug** — `/debug/tests`, `/debug/stream`, `/debug/api`. Dev-only,
  tree-shaken out of production builds.

`test/harness/` is deleted. Harness-only affordances ported into
console: Stop/Restart into the Fleet drawer, sample YAML picker into
the Editor header, SOPS status badge + vault path into the Secrets
header, scope color badges (agent=blue, team=brown, fleet=green) into
Memory, 11-row API test grid into `/debug/tests`.

**Gating** for `/debug/*` reuses the existing
`NODE_ENV === "production"` pattern from `apps/console/server.ts`:

- Client: `import.meta.env.DEV` controls whether Debug routes and the
  Debug nav group are registered. In a production build, Vite
  statically replaces this with `false`, so debug code is dead-code
  eliminated.
- Server: `apps/console/server.ts` returns 404 for `/debug/*` paths
  when `IS_PROD === true` (defense in depth — prevents the SPA index
  fallback from serving a shell that would just redirect).

No new env var introduced. No daemon-side gating changed.

## Consequences

**Positive:**

- One web UI to maintain. No drift between console and harness.
- Clearer cognitive model per nav group — end users, operators, and
  developers each see only what they need.
- Debug tooling (raw SSE, API playground, test grid) is preserved as
  first-class dev affordances instead of fading into a separate app.
- `import.meta.env.DEV` is free — no flag framework needed.

**Negative / trade-offs:**

- Harness's "works without a build" quality is gone. The console
  requires vite dev server or a prior build. If zundd is up but the
  console is broken, there's no zero-build fallback (mitigated by
  `curl` against `/v1/*`).
- Old URLs (`/fleet`, `/editor`, `/secrets`, `/events`) redirect for
  one grace period, then get removed.

**Migration:**

- `test/harness/` removed in the same PR. `test/` is now an empty
  directory (only `test/harness/` existed); removed too.
- Root `package.json` script `dev:harness` removed.
- `CLAUDE.md`, `docs/reference/guides/testing.md`,
  `HOW-TO-CLAUDE-CODE.md`, `.editorconfig` updated to point at
  `apps/console/` and drop harness references.

## Unresolved

- Whether `/debug/samples` deserves a dedicated route or stays as the
  sample picker dropdown in Editor. For now only the dropdown exists;
  no separate Samples route.
- Deep-linking to specific debug tests (e.g. `/debug/tests#health`) —
  not implemented; revisit if the test grid grows.
