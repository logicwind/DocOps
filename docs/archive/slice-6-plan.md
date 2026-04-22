# Slice 6: Skills, Tools, Memory — Implementation Plan

## Context

Slices 1-4 built the daemon, fleet parsing, container lifecycle, and agent chat. Agents can be created, messaged, updated, and destroyed — but they're "dumb": no skills, no memory, no fleet awareness. Slice 6 makes agents capable by adding skill provisioning, a fleet-aware Pi extension, and a memory system (facts + working memory). All testable via the existing dev harness.

Key architectural decisions from brainstorming:
- **No Mastra** — Pi owns conversations (JSONL), we build a simple SQLite fact store
- **Two DBs** — `sessions.db` (ephemeral index of Pi JSONL, GC'd) and `memory.db` (permanent facts + working memory)
- **Agent-controlled scoping** — agent decides scope (agent/team/fleet) on save and search
- **Async embeddings** — optional, never blocks response, ollama/openai/none
- **Session retention** — configurable (default 7d), facts survive GC

---

## New Files

### Skills

| File | Purpose |
|------|---------|
| `src/skills/loader.ts` | Validate skill directories, parse SKILL.md frontmatter (clean-room from `experiments/15-agent-skills/src/loader.ts`) |
| `src/skills/provisioner.ts` | Copy local skills to `~/.zund/data/skills/{name}/`, mount readonly into containers via `incus/devices.ts` |

### Pi Extension

| File | Purpose |
|------|---------|
| `src/pi/extension.ts` | Generate `zund-fleet.ts` extension content as string. Tools: `zund_fleet_status`, `memory_save`, `memory_search`, `working_memory_update`. Events: `before_agent_start` (fleet context), `session_start` (state reconstruction). Reads `ZUND_API_URL` env var. (Clean-room from `experiments/08-pi-extension-dev/src/extension.ts`) |
| `src/pi/extension-writer.ts` | Write generated extension into container at `/root/.pi/agent/extensions/zund-fleet.ts` via `incus exec` |

### Memory

| File | Purpose |
|------|---------|
| `src/memory/db.ts` | `MemoryDb` class using `bun:sqlite`. Tables: `facts` (id, agent, content, scope, created_at, embedding), `facts_fts` (FTS5), `working_memory` (agent, scope, content, updated_at). Methods: `saveFact`, `searchFacts`, `listFacts`, `getWorkingMemory`, `setWorkingMemory` |
| `src/memory/embeddings.ts` | Background embedding queue. Reads config from fleet defaults. Supports `provider: none/ollama/openai`. Ollama adapter: thin wrapper calling `POST /api/embed`. Enqueues on `saveFact`, processes batch every 2s, updates embedding BLOB. No-op when provider is `none`. |

### Sessions

| File | Purpose |
|------|---------|
| `src/sessions/indexer.ts` | `SessionIndexer` class using `bun:sqlite` on `sessions.db`. Scans `~/.zund/data/sessions/{agent}/` for JSONL files, indexes metadata (session ID, agent, first message, message count, timestamps, file hash). Methods: `indexAgent`, `listSessions`, `getSessionMessages` |
| `src/sessions/gc.ts` | `runSessionGC(retentionDays)`. Deletes JSONL files older than retention, removes matching rows from `sessions.db`. Never touches `memory.db`. |

### Tests

| File | Purpose |
|------|---------|
| `test/unit/skills-loader.test.ts` | SKILL.md parsing, missing file/dir errors, frontmatter validation |
| `test/unit/memory-db.test.ts` | Fact CRUD, FTS5 search, scope filtering, working memory CRUD (in-memory SQLite) |
| `test/unit/extension.test.ts` | Generated extension contains expected tools, events, API URL |
| `test/unit/session-indexer.test.ts` | JSONL indexing with temp dirs, session listing |
| `test/unit/session-gc.test.ts` | Retention-based file deletion |
| `test/fixtures/skills/sample-haiku/SKILL.md` | Test fixture for skill validation |

---

## Modified Files

### `src/fleet/executor.ts`
- Import `provisionSkills` and `writeExtension`
- In agent CREATE block: resolve skills from fleetState, call `provisionSkills()`, call `writeExtension()`
- Pass `ZUND_API_URL` env var to container

### `src/pi/launcher.ts`
- Add `skills?: Array<{ name: string; hostPath: string }>` to `LaunchAgentConfig`
- Add `envVars?: Record<string, string>` to `LaunchAgentConfig`
- Mount each skill dir readonly after sessions/workspace mounts
- Set env vars in container config

### `src/api/server.ts`
- Add `MemoryDb` and `SessionIndexer` to `AppState`, instantiate in `createState()`
- New routes:
  - `POST /v1/agents/:name/memory` — save fact `{ content, scope }`
  - `GET /v1/agents/:name/memory?q=...&scope=...` — search facts (FTS5)
  - `GET /v1/agents/:name/working-memory` — get working memory
  - `PUT /v1/agents/:name/working-memory` — update `{ content, scope }`
  - `GET /v1/agents/:name/sessions` — list indexed sessions
  - `GET /v1/agents/:name/skills` — list mounted skills
- Enhance apply preview: validate skill directories + SKILL.md

### `src/fleet/validator.ts`
- Validate `source.type: local` skills have `source.path` set
- Validate skill path resolves to directory with valid SKILL.md

### `src/fleet/types.ts`
- Add `memory` config type to defaults (embedding provider/model/url)
- Add `sessions` config type (retention)
- Add `working_template` to role resource

### `test/harness/public/index.html`
- **Memory tab** (4th tab): agent selector, fact save form (textarea + scope dropdown + save button), fact search (input + scope filter + results), working memory viewer/editor
- **Agents tab enhancement**: skills list per agent, session list per agent

---

## Build Order

### Wave 1: Pure modules (parallel, no deps)
1. `skills/loader.ts` + test
2. `memory/db.ts` + test
3. `pi/extension.ts` + test

### Wave 2: Integration (depends on Wave 1)
4. `skills/provisioner.ts` — uses loader + `incus/devices.ts`
5. `pi/extension-writer.ts` — uses extension.ts + `incus/containers.ts`
6. `memory/embeddings.ts` — uses db.ts
7. `sessions/indexer.ts` + test
8. `sessions/gc.ts` + test

### Wave 3: Wiring (depends on Wave 2)
9. Modify `pi/launcher.ts` — skills + env vars in config
10. Modify `fleet/executor.ts` — call provisioner + extension writer
11. Modify `fleet/validator.ts` — skill path validation
12. Modify `api/server.ts` — new routes, wire MemoryDb + SessionIndexer

### Wave 4: Harness
13. Modify `test/harness/public/index.html` — Memory tab, skills display, sessions view

---

## API Endpoints

| Method | Path | Body / Query | Response |
|--------|------|-------------|----------|
| `POST` | `/v1/agents/:name/memory` | `{ content, scope }` | `{ id, saved: true }` |
| `GET` | `/v1/agents/:name/memory` | `?q=...&scope=agent` | `{ facts: [{ id, content, scope, created_at }] }` |
| `GET` | `/v1/agents/:name/working-memory` | `?scope=agent` | `{ content, scope, updated_at }` |
| `PUT` | `/v1/agents/:name/working-memory` | `{ content, scope }` | `{ updated: true }` |
| `GET` | `/v1/agents/:name/sessions` | — | `{ sessions: [{ id, first_message, message_count, created_at }] }` |
| `GET` | `/v1/agents/:name/skills` | — | `{ skills: [{ name, description, has_resources }] }` |

---

## Verification

1. **Skills**: Apply fleet with `kind: skill` + agent referencing it. `incus exec zund-<agent> -- ls /skills/<name>/SKILL.md` succeeds. Chat with agent, it can reference skill content.

2. **Extension**: `incus exec zund-<agent> -- cat /root/.pi/agent/extensions/zund-fleet.ts` exists. Ask agent "what tools do you have?" — sees `zund_fleet_status`, `memory_save`, `memory_search`. Call `zund_fleet_status` — returns real fleet data.

3. **Memory**: Chat with agent, tell it to remember something. It calls `memory_save`. Search via harness Memory tab — fact appears. Restart agent — search again, fact still there. Save fact with `scope: "team"`, search from different agent with `scope: "team"` — found. Search with `scope: "agent"` — not found (correct isolation).

4. **Working memory**: Set template in role YAML. Agent fills in fields over conversation. New session — working memory injected in context, agent knows prior context without searching.

5. **Sessions**: `GET /v1/agents/:name/sessions` returns session list. After GC (set retention to 0), old sessions removed. Facts survive.

6. **Harness**: Memory tab saves/searches facts. Working memory viewer shows current state. Agent detail shows skills + sessions.
