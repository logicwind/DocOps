---
id: "0011"
title: Artifact storage — content-addressed local, pluggable interface
date: 2026-04-16
status: accepted
implementation: partial
supersedes: []
superseded_by: null
related: ["0015"]
tags: [state, artifacts, l2]
---

# 0011 · Artifact storage — content-addressed local, pluggable interface

Date: 2026-04-16
Status: accepted
Related: ADR 0015 (L2 pluggability)

## Context

Skills running inside agent containers produce binary outputs — audio
(TTS narrator), images, documents, arbitrary files. Before this was
addressed, a skill that wrote `/tmp/narration-*.mp3` inside its container
produced nothing the user could retrieve. The file never left the sandbox.

Four requirements shaped the decision:

1. **Explicit emission** — scanning container filesystems for "new files"
   is race-prone and semantically ambiguous. Skills should declare what
   they're producing.
2. **Deduplication** — a skill that re-generates the same output shouldn't
   consume storage twice.
3. **Pluggable backend** — local filesystem is fine for v0.3; S3/MinIO is
   needed for commercial/hosted deployment without code changes.
4. **Policy enforcement** — size limits, allowed kinds, retention (TTL)
   should be enforced at the store level.

## Decision

**Explicit emission via Pi tool.** Skills call
`zund_emit_artifact({ path, kind, label, mimeType? })`, registered in the
Pi extension alongside `memory_save` etc. No filesystem scanning.

**Content-addressed storage.** SHA-256 of file bytes is the canonical ID.
Free dedup across all agents.

**Storage layout:**
```
~/.zund/data/artifacts/blobs/<sha[0:2]>/<sha>   # blob contents
memory.db:artifacts                              # metadata (label, agent, kind, MIME, TTL)
```

**Pluggable `ArtifactStore` interface.** Only `LocalArtifactStore` ships
in v0.3. S3/MinIO drops in behind the same interface later.

**Policy under `kind: defaults`.** Size, retention, and allowed kinds
inherit through the fleet defaults mechanism. Per-agent overrides merge
on top.

**Event plumbing reuses `tool_execution_end`.** The tool's return value
carries `{ artifact: { id, url, mimeType, label, kind, size } }`. No new
SSE event type was added.

**Taxonomy:**
- `ArtifactKind`: `text | audio | image | video | document | binary`
- `document` covers rich formats (PDF, markdown, HTML, docx, pptx, xlsx)
- MIME is authoritative for rendering; kind is a taxonomy hint

**Defaults when no `artifacts:` block is declared:**
```
enabled: true
maxSizeMb: 25
retention: 7d
allowedKinds: [text, audio, image, video, document, binary]
```

## Consequences

**Makes easier:**

- Skills produce binary output as a first-class concept, not a
  tmpfile-and-pray pattern.
- Dedup is free.
- Future S3 backend is drop-in.
- Retention / size policy can be declared at the fleet level, not re-implemented
  per skill.

**Makes harder:**

- Content-addressing means filenames are opaque hashes. Label field carries
  human-meaningful names.
- The artifact tool must be trusted inside the container. Mitigated by
  size limits and kind allowlists.
- TTL sweeper must run — one more background task to monitor.

## Implementation notes

- Interface at `packages/daemon/src/artifacts/store.ts`.
- Local impl at `LocalArtifactStore`.
- TTL sweeper at `packages/daemon/src/artifacts/sweeper.ts`.
- REST endpoints at `packages/daemon/src/api/artifacts-routes.ts`.
- Pi tool registered at `packages/daemon/src/pi/extension.ts` (moves under
  ADR 0003).
