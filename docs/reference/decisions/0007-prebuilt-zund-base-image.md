---
id: "0007"
title: "Pre-built `zund/base` image for fast cold start"
date: 2026-04-16
status: accepted
implementation: done
supersedes: []
superseded_by: null
related: ["0004", "0005"]
tags: [substrate, image, performance]
---

# 0007 · Pre-built `zund/base` image for fast cold start

Date: 2026-04-16
Status: accepted
Evidence: POC 13 (`experiments/13-prebuilt-incus-image/`)

## Context

A fresh agent container needs Pi, Bun, and their dependencies installed. Doing
this at first-boot time means every new agent takes ~150 seconds to become
ready — unacceptable.

## Decision

Publish a pre-built Incus image (`zund/base`) with Pi, Bun, and required
system packages baked in. Agent containers launch from this image.

**Size:** ~407MB.

**Cold start measurement (POC 13):**
- From `zund/base`: **2.8s** to agent-ready
- From scratch + install: **~150s** to agent-ready

The 407MB size is worth the 50× speedup.

## Consequences

**Makes easier:**

- Agents feel instant. Users don't perceive a "waiting for setup" phase.
- Ephemeral agents (ADR 0004) are genuinely fast — 228ms clone + 2.8s start
  is still under 5s total.
- Reproducible environment — every agent starts from a known-good image.

**Makes harder:**

- Operational dependency: image must be available (registry, local cache).
- Image versioning needs a story. A stale image diverges from the daemon
  that expects it.
- 407MB is ~6× the size of a barebones Ubuntu image. First-time pull is
  noticeable on slow links.

## Implementation notes

- Image builder lives in `apps/daemon/src/incus/build-image.ts`. Reuses the
  daemon's production `IncusClient` — no `experiments/` imports.
- Per-agent Pi fleet extensions are written at launch via
  `packages/plugins/runtime-pi/src/extension-writer.ts`; the image does
  **not** bake them in (they are parameterised per agent).
- Image tag follows the daemon's breaking-change series — pre-1.0 uses
  `MAJOR.MINOR` (daemon `0.3.x` expects `zund/base:0.3`), `>=1.0` uses
  `MAJOR`. Computed in `apps/daemon/src/version.ts`; daemon boot logs the
  expected alias and an advisory pre-flight check.
- User-facing surface: `zund image build`, `zund image list`,
  `zund image rm`. Server endpoints: `POST /v1/images/base:build` (SSE
  progress), `GET /v1/images`, `DELETE /v1/images/:alias`.
- Distribution is local-build-only in v0.3. Future `push`/`pull`/`ensure`
  verbs can be slotted into the same `zund image <verb>` surface without
  breaking callers — the daemon config field `baseImage.source` exists as
  a single extension point for that work (see ADR discussion in docs/roadmap/next.md).
