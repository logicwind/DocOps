# Near-term Priorities

Forward-looking parking lot. Items are bullet-level, not specs. When an
item grows into active work, it becomes the next `roadmap/current.md`.

See `roadmap/vision.md` for strategy.
Active slice: **`roadmap/current.md`** — the magic path (foundation +
capability pivot: ADRs 0022 impl, 0027, 0028, 0029, 0030, 0023 Phase 1).

---

## Parallel lane — not blocking current

- **ADR 0020 Phase 3** (`memory-postgres` contrib plugin). Continues in
  a separate session / branch. It only touches `packages/plugins/` +
  docs and does not overlap with the magic-path surface. When it lands,
  it proves the plugin seam and ships a first-pass plugin author's
  guide at `docs/reference/guides/plugin-authoring.md`.

---

## After the magic path lands

In roughly this order. Each item grows into a `current.md` when it
starts.

1. **ADR 0023 Phase 2 — LLM dispatcher.** Capability index builder
   (reads fleet YAML + pack metadata, emits serialized doc). Small-model
   dispatcher loop: poll queue → `dispatching` → match or `pending`.
   Pending reprocessor on `zund apply`. Lights up the `dispatching` +
   `pending` states on the wire.

2. **Remaining four v1 packs** (ADR 0030). Ship after `github-workflow`
   + `research-primitives` prove the mechanism in the magic slice:
   - `productivity-gws` (Gmail + Calendar + Drive MCP)
   - `team-ops` (Linear + Notion + Slack MCPs)
   - `docs-io` (PDF, OCR, long-doc summarization)
   - `browser-automation` (`playwright-mcp` via the sidecar)

3. **Channel adapters — Slack / Telegram / WhatsApp** (ADR 0022 §10).
   Re-use Hermes gateway's progressive-edit + rate-limit pattern against
   the `UIMessage` + `data-z:*` wire. Pairs with `team-ops` pack.
   Enables the "send a message from WhatsApp, agent replies there"
   demo.

4. **ADR 0023 Phase 3 — Triggers.** Cron scheduler (in-process,
   reads `TriggerConfig.cron` from fleet YAML). Webhook endpoint
   (`POST /v1/triggers/:name`). Agent-chain events. All feed the queue.
   `scheduled` state activates.

5. **`zund auth setup` wizard.** OAuth + API-key collection for pack
   secrets. Flagged in ADR 0029 and ADR 0030. Without this, packs that
   need OAuth (GWS, Slack, Notion) are rough to install.

6. **ADR 0018 Phase 2 — Hermes runtime plugin.** Second runtime
   validates the ADR 0028 contract (runtime_config + fleet_capabilities
   bridging) against a non-Pi runtime. Also subsumes Hermes as a fleet
   member per the vision.

7. **ADR 0023 Phase 4 — capacity-aware routing + ephemeral spawn +
   blocked state.** Runtime workload tracking, ephemeral Spot spawn for
   unmatched tasks, `blocked` state for agents waiting on external
   input (human approval, rate limit, tool timeout).

8. **ADR 0031 — Media capabilities** (STT / TTS / Vision / Image-gen).
   Currently draft. Promote when the first demo actually needs voice
   or multimodal egress.

---

## Prereq refactors (in flight; don't block the slice above)

- **`api/server.ts` route-group extraction.** Partial: fleet/image/debug
  and memory/sessions routes are already extracted (see recent commits).
  Remaining: agent, tasks, secrets, capabilities routes. Incremental;
  can slot in any time.

- **Wire `cloneEphemeral()` from POC 06** into the daemon's agent
  launcher. Currently experiment-only. Prereq for ephemeral task-driven
  agents in Phase 4.

### Already done (do not re-add)

- `fleet/executor.ts` split → `fleet/reconciler.ts` +
  `agents/launcher.ts` — shipped.
- Runtime interface extraction (ADR 0003) — subsumed by ADR 0020 Phase 2.
- Canonical stream protocol (ADR 0002) — superseded by ADR 0022.
- `MemoryStore` / `SecretStore` / `ArtifactStore` / `SessionStore`
  interfaces — subsumed by ADR 0020 Phase 2.
- Daemon API auth + transport hardening (ADR 0026) — shipped.

---

## CLI plugin-decoupling refactor (carried)

Today `apps/cli/src/commands/secret/{set,get,list,remove}.ts` import
`@zund/plugin-secrets-age-sops/vault` directly, bypassing the plugin
registry. If an operator swaps the active impl via
`~/.zund/plugins.yaml` (e.g. to `secrets-env`), the CLI silently
writes to a backend the daemon doesn't read. `rotate.ts` is the model
— uses the daemon's HTTP route, knows nothing about SOPS or age.

- Rewrite `set/get/list/remove` to use daemon routes
  (`secrets-routes.ts` implements them in full).
- Drop `@zund/plugin-secrets-age-sops` from `apps/cli/package.json`.
- Lift `requireAppliedFleet(ctx)` from `rotate.ts` into `_helpers.ts`.
- Audit `apps/cli/**` and `apps/console/**` for stray `@zund/plugin-*`
  imports — none should exist.

Unblocked by ADR 0026 (auth + transport hardening).

---

## Open questions (carried)

- **G. Per-agent memory storage strategy.** Three shapes (logical
  split daemon-mediated, bind-mount daemon-writer, full-local-ownership).
  Current lean: option 1. Revisit after memory/skills integration
  tests prove the shared-DB shape works end-to-end. Interface from
  ADR 0016 keeps the decision reversible.

- **I. CLI follow-ups.** Six deferred items from Slice 7:
  1. Bundle sops + age binaries for npm / brew installs.
  2. GitHub Release pipeline for binary distribution.
  3. TUI for `zund chat` (Ink or blessed). Defer until user complaint.
  4. `zund agent create` / `delete` wizards. Defer until new-user
     patterns emerge.
  5. `zund secret audit` static analysis command.
  6. `zund apply` no-diff case: re-resolve secrets even when plan is
     empty (or dedicated `zund secret rotate`).

- **K. Multi-user agent sessions.** Four shapes (single-user default,
  multi-pi-per-container, ephemeral-per-session, queue-and-switch).
  Current thinking: option 1 default; option 2 as opt-in
  `concurrency: N` when a use case appears; option 3 for commercial
  multi-tenant; option 4 rejected. Blocker for multi-tab console
  usage. **May deserve its own ADR** when the use case crystallizes.

---

## Smaller items

- **Shared types package** (`@zund/types`) to dedupe transport types
  duplicated in `apps/cli/src/transport/types.ts` and
  `apps/console/src/lib/types.ts`.
- **Container naming becomes runtime-specific** (currently
  `zund-${name}` is hardcoded in the executor).
- **Per-fleet `<fleetDir>/plugins.yaml` overrides** — deferred three
  times through ADR 0020 Phase 2; wire the lifecycle once a real user
  demands fleet-scoped plugin config.
- **Fleet-bridge prompt advertising** — the Pi `before_agent_start`
  handler lists memory/artifact tools unconditionally. Make it driven
  by `boundKinds` once someone actually runs with a reduced plugin set.
- **`tools:` field deprecation path.** ADR 0028 supersedes the
  top-level `tools:` field in favor of `runtime_config: { extensions }`.
  After the magic slice ships, remove the old field from parsers and
  docs (not before — would break fleets mid-migration).

---

## Out of scope (explicitly)

- Multi-node Zund clusters. Single-host is the design target;
  federation is commercial.
- Kubernetes integration. Incus is the substrate (ADR 0004).
- Native integrations for what MCP already covers (GitHub, Linear,
  Notion, Slack, Google Workspace, Playwright). Route via MCP sidecar
  per ADR 0029.
- Alternative agent runtimes (VM, SSH) as v1 deliverables — the
  *interface* is the v1 deliverable (ADR 0003); a second
  implementation (Hermes) is post-magic-slice work.
- Replacing Pi as the default runtime (accepted per ADR 0005).
- Own skill-marketplace / skill-discovery product. Skills live inside
  packs (ADR 0030); pack distribution is npm. We do not run a
  registry.
