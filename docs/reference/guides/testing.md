# Testing Strategy

How Zund is tested. Enforced by project convention — every module gets
tests alongside it, not after.

---

## Test types

```
apps/daemon/test/
├── unit/           bun test, pure logic, no infrastructure, fast
├── integration/    bun test, needs running zundd + Incus
└── fixtures/       shared test data (skills, fleet YAML, JSONL sessions)

apps/console/   live web dashboard at localhost:3000 (Work / Admin / Debug tabs)
test/smoke/         end-to-end smoke tests with real model providers
scripts/smoke-runs/ preserved smoke test artifacts
```

Samples under `samples/` provide reference fleet configurations used by
integration and smoke tests (e.g., `samples/test-full/` exercises every
resource kind).

---

## Unit tests

**Scope:** pure logic, no infrastructure, no network.

Every module gets tests alongside it: parser, differ, config loader,
schema validator, SSE encoder, RPC session registry, memory DB, artifact
store, secret resolver — all tested without touching Incus or spawning
containers.

```
apps/daemon/test/unit/
├── fleet/           parser, differ, validator, defaults
├── memory/          SQLite store, embeddings queue
├── artifacts/       content-addressing, policy, sweeper
├── secrets/         age+sops resolver, vault
├── sessions/        indexer, GC
├── skills/          loader, provisioner
├── pi/              extension writer, RPC session registry
└── incus/           client (with mocked Bun fetch)
```

**Principles:**

- No mocks of Incus, Ollama, model providers. Either it's pure logic
  (test it directly) or it's integration (test it for real).
- Test fixtures live under `test/fixtures/`, shared across suites.
- Runs in under 30 seconds on a laptop.

---

## Integration tests

**Scope:** scripted scenarios against live zundd + live Incus.

```
apps/daemon/test/integration/
└── e2e/             create → apply → message → verify → destroy
```

**Requirements:**

- Incus available on the test host (Linux or macOS + Colima)
- Pre-built `zund/base` image pulled
- API keys in a test vault for provider calls (unless using Ollama)

**Run:**

```bash
bun test:integration
```

Integration tests are not run on every commit — they're run on PRs and
before releases. They depend on external state (Incus, networks, model
providers) and will be flaky otherwise.

---

## Dev dashboard

Not a test — a *development tool*. Live web console at `:3000`, used
alongside zundd during feature work. Organized into three groups:

- **Work** — Chat, Memory
- **Admin** — Fleet, Editor, Secrets, Events
- **Debug** (dev-only; hidden in production builds) — Tests grid, raw SSE
  stream, API playground

```
apps/console/
├── src/                vite + react + tailwind
└── server.ts           Bun server that serves the app and proxies to zundd
```

**Run:**

- Terminal 1: `bun --filter @zund/daemon dev`
- Terminal 2: `pnpm --filter @logicwind/zund dev` (browser at localhost:3000)

The Debug group is gated on `NODE_ENV !== "production"` via Vite's
`import.meta.env.DEV`, and `/debug/*` paths are 404'd server-side in prod.

---

## Smoke tests

End-to-end tests that exercise the full stack with real model providers.

```
test/smoke/
└── ...              full pipeline: zund CLI → zundd → Pi → OpenRouter → response
```

Run manually during release verification:

```bash
bun test:smoke
```

Artifacts from each run are preserved under `scripts/smoke-runs/` for
post-hoc inspection.

---

## CI considerations

- **Unit tests**: every PR, mandatory gate.
- **Integration tests**: PR-level, requires Incus-enabled runner.
- **Smoke tests**: release-level, requires provider API keys.
- **Console dashboard**: not run in CI — developer tool only.

---

## Related

- [`reference/daemon.md`](../daemon.md) — daemon internals under test
- Project code principles: see project root `CLAUDE.md`
