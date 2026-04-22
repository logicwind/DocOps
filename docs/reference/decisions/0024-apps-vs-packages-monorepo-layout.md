---
id: "0024"
title: "Monorepo layout — apps/ for deployables, packages/ for libraries"
date: 2026-04-17
status: accepted
implementation: done
supersedes: []
superseded_by: null
related: ["0001", "0020", "0021", "0022"]
tags: [monorepo, layout, tooling, repo-structure]
---

# 0024 · Monorepo layout — apps/ for deployables, packages/ for libraries

Date: 2026-04-17
Status: accepted
Related: ADR 0001 (four-layer architecture), ADR 0020 (plugin architecture),
ADR 0021 (console consolidation), ADR 0022 (stream protocol)

## Context

Today the repo has both an `apps/` and a `packages/` tree, with contents
split by historical accident rather than a rule:

```
apps/
  docs/                   Next.js + Fumadocs site
packages/
  cli/                    `zund` binary; published to npm
  console/                Vite SPA + server.ts (the web console)
  core/                   @zund/core — contracts, registry, loader
  daemon/                 `zundd` server binary
  plugins/
    artifacts-local/      @zund/plugin-*  (libraries loaded by daemon)
    memory-sqlite/
    runtime-pi/
    secrets-age-sops/
    secrets-env/
```

`pnpm-workspace.yaml` globs both trees, so tooling works regardless of
which folder a package lives in. There is no functional bug — this is
purely an organization decision.

Three problems show up in practice:

1. **New-contributor orientation is muddy.** `packages/` mixes leaf
   deployable binaries (`daemon`, `console`, `cli`) with libraries
   (`core`, `plugins/*`). A reader can't tell from the tree which
   packages are "the things you run" vs "the things that get imported."

2. **CLAUDE.md violation hiding in the layout.** `packages/cli` imports
   from `@zund/daemon/src/secrets/vault.ts` in four files under
   `packages/cli/src/commands/secret/`. This violates CLAUDE.md's
   explicit rule: *"packages/cli — thin HTTP wrapper, zero business
   logic."* The layout doesn't cause the violation, but having both
   binaries in `packages/` makes the cross-import feel natural instead
   of alarming. A clean apps/lib split makes the wart obvious: apps
   should never import other apps.

3. **Phase 2 of ADR 0020 doubled down on packages/plugins/.** Plugins
   are genuine libraries (imported by the daemon at boot). They belong
   in `packages/`. But they now share the tree with three deployable
   apps, blurring what `packages/` means.

This ADR locks a rule so future additions land in the right place by
default.

## Decision

### The rule

```
apps/        Deployable end-user artifacts. Not imported by other code.
packages/    Libraries that get imported. No runtime entry point.
```

If a workspace package has any of these, it's an **app**:

- A `bin` entry that end users invoke (`zund`, `zundd`).
- A dev server / build → deployed artifact (`next build`, `vite build`).
- A long-running process (`bun src/index.ts` as the product).

If a workspace package is consumed via `import ... from "@zund/..."` by
other packages and has no runtime entry point of its own, it's a
**library**.

### Target layout

```
apps/
  cli/             @zund/cli          (was packages/cli)
  console/         @zund/console      (was packages/console)
  daemon/          @zund/daemon       (was packages/daemon)
  docs/            docs               (already here)
packages/
  core/            @zund/core
  plugins/
    artifacts-local/
    memory-sqlite/
    runtime-pi/
    secrets-age-sops/
    secrets-env/
```

### Prerequisite — CLI→daemon import wart (done)

**Status:** resolved in commit `3a8b44f` (2026-04-17).

The four CLI secret subcommands
(`packages/cli/src/commands/secret/{list,remove,set,get}.ts`) used to
pull `listSecrets` / `removeSecret` / `setSecret` / `getSecret` directly
from `@zund/daemon/src/secrets/vault.ts` — a path that no longer existed
after ADR 0020 Phase 2d moved the secrets module into
`@zund/plugin-secrets-age-sops`. Under the new rule this would have been
an app importing another app.

The fix used **option 2** (shared library) — but no new package was
needed because the plugin already is a library. CLI's package.json
dropped `@zund/daemon` and added `@zund/plugin-secrets-age-sops`; each
import re-points at `@zund/plugin-secrets-age-sops/vault`. Result: CLI
no longer imports any daemon sources; `@zund/daemon` becomes a true
leaf deployable.

Option 1 (HTTP endpoints) was rejected because secret ops are
fundamentally filesystem ops on the fleet directory — `zund secret set`
works today without the daemon running, and routing through HTTP would
have forced a daemon dependency on a previously offline-friendly
command path.

### Execution order

1. ~~Fix CLI→daemon imports~~ **done** (`3a8b44f`).
2. ~~Single commit moving `packages/{cli,console,daemon}` → `apps/*`~~
   **done** alongside this ADR's acceptance (see git log for the
   rename commit). The move itself was mechanical `git mv`; the same
   commit updated `pnpm-workspace.yaml` comment, `CLAUDE.md` project
   structure block, `HOW-TO-CLAUDE-CODE.md`, `scripts/smoke-test.sh`,
   `apps/cli/scripts/build-binaries.sh`, the four
   `packages/core/src/contracts/*.ts` historical-reference comments,
   both `samples/*` READMEs, and current-state reference docs
   (`docs/reference/{daemon,architecture,runtime-protocol}.md` plus
   the three `guides/*.md`). ADR 0021's `packages/console/` refs were
   fixed inline per the Consequences block below. Accepted ADRs other
   than 0021 kept their historical paths.
3. ~~Verify~~ **done**: `pnpm install` re-linked 11 workspaces cleanly;
   `@zund/cli` unit tests green; plugin tests green; `zund secret
   --help` loads from the new path.

### Future-proofing

New workspace packages follow the rule without discussion:

- New runtime plugin? → `packages/plugins/runtime-<name>/`.
- New contrib library (e.g. `@zund/plugin-sdk` from ADR 0020 Phase 5)?
  → `packages/`.
- New deployable (future hosted control plane, agent CLI helper,
  dashboard)? → `apps/`.
- New docs site / landing page? → `apps/`.

## Consequences

**Makes easier**

- Fresh reader sees the product surface (`apps/`) separately from the
  library surface (`packages/`). Matches the Nx / Turborepo convention
  most contributors already expect.
- Cross-app imports become obviously wrong — the tree itself flags the
  CLAUDE.md violation class.
- Future ADRs that talk about "the console" or "the daemon" have a
  stable, unambiguous path. Phase 5 `@zund/plugin-sdk` publishing has
  a clear home.

**Makes harder**

- One-time churn: every external link, doc, script, and `import` path
  pointing at `packages/{cli,console,daemon}` breaks simultaneously.
  Ctrl-F fixes it, but the diff is wide.
- Git blame continuity is preserved (`git log --follow`) but noisier.
  ADR-0021's `packages/console/` references need an addendum or inline
  fix (inline preferred; ADR body is not immutable for factual path
  fixes even though decisions are).
- Contributors who bookmarked `packages/daemon/` muscle-memory need to
  re-wire. Small cost.

**Unchanged**

- `@zund/core` and `@zund/plugin-*` stay where they are — they are
  libraries by the rule. No plugin package moves.
- Workspace dep graph is identical post-move; `@zund/cli` still depends
  on `@zund/daemon`, just via a new path.
- Build, test, publish, and smoke-test commands are unaffected aside
  from path strings.

## Out of scope

- Splitting `packages/core` further (contracts vs registry). Separate
  discussion if it ever comes up.
- Moving `experiments/` (it's reference-only, stays where it is).
- Renaming packages (`@zund/daemon` stays `@zund/daemon`).
- Any TypeScript project-references restructure.
- A published `@zund/plugin-sdk` — that's ADR 0020 Phase 5, and it will
  land in `packages/` when it does.
