# Slice 8: File/Artifact Support — Plan

## Context

Skills run inside agent containers and currently cannot return binary files. The TTS narrator skill writes `/tmp/narration-*.mp3` inside its container and nothing leaves the sandbox. This slice closes that gap end-to-end: skills emit files explicitly via a Pi tool, the daemon stores them content-addressed, the CLI prints a download path, and the console renders them inline.

Zero existing code speaks this vocabulary — no `artifact|attachment|blob|media` anywhere in the codebase. The primitive is introduced in 6 independent sub-slices so each can be smoke-tested and merged before the next begins.

Execution detail (file-by-file changes, test listings, verify commands) lives in `.claude/plans/peppy-scribbling-origami.md`.

---

## Design Decisions (locked)

1. **Explicit emission via Pi tool.** Skills call `zund_emit_artifact({ path, kind, label, mimeType? })` — registered in `packages/daemon/src/pi/extension.ts` alongside existing `memory_save` / `working_memory_update`. Not filesystem scanning. Race-free, carries a friendly label, clean contract.

2. **Content-addressed storage.** SHA-256 of file bytes is the canonical ID. Free dedup. Layout: `~/.zund/data/artifacts/blobs/<sha[0:2]>/<sha>`. Metadata (label, agent, kind, MIME, TTL) in `memory.db`.

3. **Pluggable `ArtifactStore` interface.** Only `LocalArtifactStore` in v1. S3/MinIO drops in behind the same interface later without touching callers. No stub in v1.

4. **Policy under `kind: defaults`, not a new top-level kind.** `packages/daemon/src/fleet/defaults.ts` already deep-merges nested blocks recursively, so `defaults { agent: { artifacts: {...} } }` inherits field-by-field into every agent. Per-agent overrides merge on top. No parser changes needed.

5. **Event plumbing reuses `tool_execution_end`.** The tool's return value carries `{ artifact: { id, url, mimeType, label, kind, size } }`. Clients inspect the existing event by `toolName`. No new SSE event type in `packages/daemon/src/pi/rpc.ts`.

6. **CLI stale event vocabulary fixed in Slice D.** `packages/cli/src/commands/agent/chat.ts` currently handles only legacy event names and silently drops Pi's native names (`message_update|tool_execution_start|tool_execution_end|agent_end`). Fixed on the way past.

7. **Default policy (no `artifacts:` block required).** `enabled: true`, `maxSizeMb: 25`, `retention: 7d`, `allowedKinds: [text, audio, image, video, document, binary]`. One constant, one file.

8. **`ArtifactKind` is a taxonomy hint; MIME is authoritative for rendering.** Kind union: `text | audio | image | video | document | binary`. `document` covers rich formats (PDF, markdown, HTML, docx, pptx, xlsx). The console renderer switches on full MIME type first, then falls back to top-level kind. Policy enforcement (size, allowed kinds) gates at upload time.

---

## YAML Shape

### Fleet defaults (all agents inherit)

```yaml
kind: defaults
agent:
  model: claude-sonnet-4-5
  artifacts:
    enabled: true
    maxSizeMb: 25
    retention: 7d
    allowedKinds: [audio, image, document]
```

### Per-agent override (deep-merges on top of defaults)

```yaml
kind: agent
name: narrator
role: tts-narrator
artifacts:
  maxSizeMb: 10       # tighter cap for this agent
  retention: 1h       # short-lived audio
  # allowedKinds inherits [audio, image, document] from defaults
```

Fields not overridden are inherited from `kind: defaults`. If no `artifacts:` block exists anywhere, `DEFAULT_ARTIFACT_POLICY` applies.

---

## Pi Tool Signature

Registered in `packages/daemon/src/pi/extension.ts`:

```typescript
zund_emit_artifact({
  path: string,        // absolute path inside container, e.g. /artifacts/out.mp3
  kind: "text" | "audio" | "image" | "video" | "document" | "binary",
  label: string,       // human-readable filename, e.g. "narration.mp3"
  mimeType?: string,   // inferred from extension if omitted
})
```

MIME inference covers common extensions (`.mp3`, `.png`, `.pdf`, `.md`, `.docx`, etc.). Unknown extensions fall back to `application/octet-stream`.

The tool POSTs bytes from `/artifacts/<file>` to `POST /v1/artifacts` using native `fetch` (not `zundFetch` — that helper hardcodes `Content-Type: application/json` and breaks multipart). Returns:

```json
{ "artifact": { "id": "<sha256>", "url": "/v1/artifacts/<sha256>", "mimeType": "audio/mpeg", "label": "narration.mp3", "kind": "audio", "size": 42180 } }
```

---

## REST API Surface

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/artifacts` | `multipart/form-data`: `agent`, `kind`, `label`, `mimeType`, `data`. Policy-checked. Returns artifact descriptor. 413 on size cap; 415 on disallowed kind. |
| `GET` | `/v1/artifacts/:id` | Streams blob with correct `Content-Type` and `Content-Disposition: inline`. 404 on missing. |
| `GET` | `/v1/artifacts/:id/meta` | JSON metadata only (no bytes). |
| `GET` | `/v1/artifacts?agent=<name>&limit=<n>` | List artifacts for an agent. |
| `DELETE` | `/v1/artifacts/:id` | Removes metadata row. Blob unlinked only when `countReferencesToBlob == 0` (dedup safety). |

---

## CLI Surface

New subcommand group `zund artifact`:

| Command | Description |
|---------|-------------|
| `zund artifact list [--agent=name] [--json]` | Table: short ID, label, kind, size, created. |
| `zund artifact get <id> [--output=path]` | Download to `~/.zund/downloads/<label>` by default. |
| `zund artifact rm <id> [--yes]` | Delete. Confirms unless `--yes`. |
| `zund artifact open <id>` | Download then `open` / `xdg-open`. |

During `zund agent chat` and `zund agent message`, an artifact emission prints inline:

```
📎 narration.mp3 (41 KB) → ~/.zund/downloads/narrator/narration.mp3
```

The chat/message event handlers are extracted to a shared `_event-handler.ts` in Slice D.

---

## Sub-Slices

### Slice A — Types + SQLite schema

**Goal:** Introduce `ArtifactPolicy`, `ArtifactMeta`, and the `artifacts` table in `memory.db`. Zero runtime behavior change.

Adds the type interfaces (`ArtifactKind`, `ArtifactPolicy`) to `packages/daemon/src/fleet/types.ts`, extends `MemoryDb` in `packages/daemon/src/memory/db.ts` with artifact CRUD methods, and adds `zundArtifacts()` / `zundArtifactBlob(sha)` to `packages/daemon/src/paths.ts`. Schema is idempotent `CREATE IF NOT EXISTS`. No migration framework needed.

### Slice B — `ArtifactStore` interface + daemon REST endpoints

**Goal:** Server-side storage and HTTP surface. Nothing calls it yet.

Introduces `packages/daemon/src/artifacts/store.ts` (`ArtifactStore` interface + `LocalArtifactStore`), `packages/daemon/src/artifacts/policy.ts` (`DEFAULT_ARTIFACT_POLICY`, `resolveArtifactPolicy`, `validateUpload`), and `packages/daemon/src/api/artifacts-routes.ts` with the five handlers above. Wires `artifactStore` into `AppState` in `packages/daemon/src/api/server.ts`.

### Slice C — Pi extension tool + `/artifacts` mount

**Goal:** Skills can publish a file by calling one tool. Bytes flow container → daemon → CAS.

Registers `zund_emit_artifact` in `packages/daemon/src/pi/extension.ts`. Extends `LaunchAgentConfig` in `packages/daemon/src/pi/launcher.ts` to bind-mount a per-agent staging dir as `/artifacts` (rw) inside the container. Passes `artifactsStagingDir` from `packages/daemon/src/fleet/executor.ts`. Per-agent isolation by design.

### Slice D — Client-side event handling + CLI `artifact` command

**Goal:** CLI surfaces emitted artifacts inline and via a dedicated command group.

Rewrites the event dispatch loop in `packages/cli/src/commands/agent/chat.ts` and `message.ts` to handle Pi's native event names. Shared logic extracted to `packages/cli/src/commands/agent/_event-handler.ts`. Adds the four `artifact` subcommands under `packages/cli/src/commands/artifact/`. Registers the subcommand tree in `packages/cli/src/index.ts`.

### Slice E — Console inline rendering

**Goal:** The web console renders audio/image/video/PDF/markdown/HTML/office-docs inline (or as a labeled download) under assistant messages.

Adds `ArtifactDescriptor` to `packages/console/src/lib/types.ts`, extends the SSE reducer in `packages/console/src/lib/hooks/useAgentMessageStream.ts` to dispatch an `ATTACHMENT` action on `tool_execution_end` for `zund_emit_artifact`, renders attachments in `packages/console/src/components/chat/MessageBubble.tsx` via a new `AttachmentRenderer.tsx`.

Rendering precedence (full MIME first, then top-level):
1. `application/pdf` → inline `<iframe>`
2. `text/markdown` → rendered markdown card
3. `text/html` → sandboxed `<iframe sandbox="">`
4. `text/plain` / `text/csv` / `application/json` → `<pre>`
5. `application/vnd.openxmlformats-officedocument.*` → labeled download card with format icon
6. `audio/*` → `<audio controls>`
7. `image/*` → `<img>`
8. `video/*` → `<video controls>`
9. Fallback → generic download link

### Slice F — TTL sweeper + smoke test update

**Goal:** Expired artifacts collected automatically; smoke test asserts the full path works.

Adds `packages/daemon/src/artifacts/sweeper.ts` — runs every 30 minutes, calls `expiredArtifacts()`, deletes rows and blobs (blob only when `countReferencesToBlob == 0`). Wired into `startZundd()` in `server.ts`. Updates `samples/smoke-test-fleet/` (skill output path → `/artifacts`, adds fleet artifact policy) and `scripts/smoke-test.sh` (asserts narrator emits an audio artifact and it downloads successfully).

---

## Out of Scope (v1)

- No MinIO/S3 implementation — interface only; class lands later.
- No presigned URLs — local backend streams through daemon.
- No thumbnail generation.
- No per-skill policy — only per-agent + fleet defaults.
- No public-sharing flag — everything is daemon-auth-gated.
- No `zund artifact sweep` CLI command — sweeper is automatic.
- No renderer registry for arbitrary tool names — only `zund_emit_artifact` recognized specially.

---

## End-to-End Verification (post all slices)

1. `bun test` fully green across unit + integration.
2. `rm -rf ~/.zund/data/artifacts` then start a fresh daemon.
3. `scripts/smoke-test.sh` with `OPENROUTER_API_KEY` + `ELEVENLABS_API_KEY`:
   - Narrator returns inline text + `📎 narration.mp3 → ~/.zund/downloads/narrator/narration.mp3`.
   - `zund artifact list --agent=narrator` shows the row.
   - `zund artifact get <id>` downloads bytes.
4. In console, ask narrator to narrate — confirm `<audio>` element renders and plays.
5. Inspect `~/.zund/data/artifacts/blobs/<prefix>/<sha>` for CAS layout.
6. Set `retention: 1h`, wait past sweeper interval, confirm row + blob gone.
7. Re-emit same bytes from a different agent — confirm both metadata rows share one on-disk blob.
