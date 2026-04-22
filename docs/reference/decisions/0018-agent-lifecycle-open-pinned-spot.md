---
id: "0018"
title: "Agent lifecycle — Open, Pinned, Spot tiers with role-based iteration"
date: 2026-04-16
status: draft
implementation: not-started
supersedes: []
superseded_by: null
related: ["0003", "0004", "0007", "0017"]
tags: [lifecycle, agents, promotion, roles, l1, l3]
---

# 0018 · Agent lifecycle — Open, Pinned, Spot tiers with role-based iteration

Date: 2026-04-16
Status: draft
Related: ADR 0003 (runtime interface), ADR 0004 (Incus substrate), ADR 0007 (prebuilt image), ADR 0017 (humans as fleet members)

## Context

Zund's current fleet model treats every agent as long-lived and
declaratively defined in YAML — apply writes the world, the world matches
YAML. This works for reproducibility but fights two things real teams need:

1. **Self-improvement.** OpenClaw/Hermes shine because agents write skills,
   install tools, and accumulate capability from experience. A strictly
   declarative runtime blocks this or discards it on restart.
2. **Ephemeral work.** Cron-triggered or one-shot tasks need sub-second
   spawn; a persistent container per task is wasteful.

ADR 0004 already anticipated ephemeral agents via `incus copy --ephemeral`,
but the full lifecycle story isn't pinned: what's the persistent, mutable,
self-improving mode? How does drift become declarative? Who can iterate
and how is that coordinated on a team?

Without answers, three problems surface:

- Self-improving agents drift forever with no path back to declarative.
- Multiple teammates trying to "improve the researcher" collide or
  fragment into per-person copies that drift apart (OpenClaw's per-user
  isolation problem, imported into Zund).
- Operators have no mental model for "which container is the source of
  truth for this role right now."

## Decision

Agents exist in exactly one of three **lifecycle tiers**, organized
around **roles**. Terminology and constraints are normative.

### Tiers

| Tier | Count per role | Mutable filesystem? | Reproducible from YAML? | Use case |
|------|----------------|---------------------|-------------------------|----------|
| **Open** | 0 or 1 | Yes | No (accumulates drift) | Iteration, self-improvement, discovery |
| **Pinned** | 0 to N | No | Yes (from published image) | Production, team consumption |
| **Spot** | 0 to ∞ | No | Yes (clone from Pinned image) | One-shot tasks, cron-triggered work |

### Roles

A **role** is a named capability the fleet provides (`researcher`, `writer`,
`reviewer`). Each role declares the image Pinned/Spot agents run from, the
skills they use, and — optionally — who currently holds the role's Open
iterator.

```yaml
# fleet/roles/researcher.yaml
kind: Role
name: researcher
image: zund/researcher:v3         # what Pinned and Spot run
skills: [summarize, search-web]
packages: []                       # OS packages baked into image
open:
  holder: null                    # currently unheld
  base: zund/researcher:v3        # fork point when next claimed
snapshot:
  schedule: "0 * * * *"           # hourly when Open is held
  retain: 7d
```

### Invariants

- **One Open agent per role, ever.** Enforced by the daemon. Claiming Open
  for a role that already has a holder fails unless `--force-takeover` is
  passed (which snapshots and releases the previous holder).
- **Pinned agents are immutable at runtime.** `incus exec ... apt install`
  is blocked or transparently no-op'd. Drift is not supported; restart
  reconciles.
- **Spot agents never emit proposals or persist state beyond the task.**
- **Open is optional.** A role can exist with only a Pinned image and no
  iterator. Most mature roles live this way most of the time.

## Phased implementation

The full design is large. Ship it in phases so value lands quickly and
the proposal stream (biggest surface) can inherit from a working base.

### Phase 1 — Open → Pinned publishing (MVP)

**Goal:** a role can be iterated in Open mode, then the Open container's
state published as a new image that Pinned consumers upgrade to.

**In scope:**

- `Role` resource kind in fleet YAML.
- Open/Pinned/Spot tier tracking in daemon state.
- `zund role <name> open [--claim]` — create Open container from role's
  current image; record holder.
- `zund role <name> publish [--tag vN]` — `incus publish` the Open
  container → new image; update role YAML's `image:` field.
- `zund role <name> release` — stop Open container, snapshot, clear
  holder.
- `zund apply` handles rolling-restart of Pinned agents on image change.
- Spot agents spawn from current role image via `incus copy --ephemeral`.
- Manual iteration: user messages Open agent; any state changes are
  whatever the runtime (Pi, OpenClaw, Hermes) does natively. No watcher,
  no proposals — just "the container is yours, edit it how you want."
- Promote = full-image publish only. Skill/capability-level promotion
  deferred to Phase 2.

**Out of scope (explicitly):**

- Filesystem watcher / inotify integration.
- Typed proposal events (SkillProposal, PackageProposal, etc.).
- Auto-approval policy engine.
- Hot-apply of individual packages without image rebuild.
- Multi-person shared Open sessions (single-holder only in Phase 1).

**MVP user story:**
> Alice claims Open on the `researcher` role. She messages the agent for
> a week, it writes skills, she installs `wrangler` via bash. She runs
> `zund role researcher publish --tag v4`. Incus publishes the container
> as `zund/researcher:v4`. Bob's Pinned researchers rolling-restart onto
> v4. His agents now have wrangler and the new skills. Alice releases
> Open.

### Phase 2 — Proposal stream

**Goal:** surface individual skills/packages/MCP servers as they happen
in the Open container, so teammates can promote discoveries incrementally
without waiting for a full image publish.

**Scope:**

- `fleet/runtimes/<runtime>/<role>/` host-mounted workspace volume.
- Filesystem watcher emits typed proposals:
  `SkillProposal`, `MCPProposal`, `ScriptProposal`, `PackageProposal`.
- Tool-call introspection for shell commands (`apt install X` →
  `PackageProposal`).
- Periodic snapshot-diff for drift detection (hourly by default).
- Proposal UI in console; approve/deny actions.
- Auto-approval policies in YAML (allowlist by kind + source).
- Hot-apply for approved proposals (install live into Pinned containers,
  queue image rebuild in background).

Phase 2 is additive. Phase 1 deployments keep working without it; users
who skip Phase 2 still get full-image publishing.

### Phase 3 — Scale & governance (deferred)

- Multi-holder Open sessions (pair programming on a role).
- Role-level RBAC: who can claim Open, who can publish, who can deny
  proposals.
- Cross-fleet role libraries (publish a role image to a team registry).
- Role forking / branching for parallel experiments (`researcher-beta`).
- Integration with ADR 0017 human fleet members for approval routing.

## Challenges and open questions

### Phase 1

- **Image versioning policy.** Auto-increment (`v1`, `v2`, ...), semver,
  or content-hash? Recommendation: auto-increment integer tag + optional
  named tags (`v4`, `v4-stable`).
- **Broken-image rollback.** If v4 regresses, how do Pinned agents
  revert? Rollback path = update `fleet/roles/researcher.yaml` to
  `image: zund/researcher:v3`, re-apply, Pinned rolling-restart onto v3.
  Keep last N images on disk (configurable, default 5).
- **Disk cost.** Each publish = full rootfs image. At 2GB × 5 retained
  versions × 6 roles = 60GB. Need `zund images gc` or age-based cleanup.
- **Open holder identity.** Today Zund has no user model. Hardcode
  `holder: <hostname>` in Phase 1; defer user auth to Phase 3. The
  constraint (one holder) is still enforceable by the daemon.
- **Pinned restart coordination.** If a Pinned agent is mid-conversation
  during image upgrade, do we drain or force? Recommendation: drain with
  a configurable timeout (default 30s), then force. Session continuity is
  a Phase 3 concern (requires session checkpoint/restore).
- **Spot + stale image.** Spot agents spawn from Pinned image; if Pinned
  was just upgraded, Spot picks up new image on next spawn.
  Acceptable — Spot is stateless.
- **Open + Pinned simultaneously on same host.** Both run — no conflict;
  they're separate containers. Messages route to whichever tier the
  caller specifies (default: Pinned). Alice talking to Open researcher
  and Bob talking to Pinned researcher are fully isolated.
- **Naming collisions.** Role `researcher` produces containers like
  `zund-role-researcher-open` and `zund-role-researcher-pinned-<i>`.
  Pin naming convention in daemon so humans can grep.
- **What if no Pinned image exists yet?** A freshly-declared role starts
  with `image: zund/base:default` (or similar). First publish creates v1.
  Open-first workflow is the bootstrap case.

### Phase 2

- **Watcher fidelity.** inotify catches only what the agent writes to
  mounted paths; system-wide installs miss. Mitigation stack (A + B + C
  + D) documented in discussion transcripts — hybrid fast-path + slow
  snapshot-diff. Full-image publish remains the 100% escape hatch.
- **Denial semantics.** For Phase 2, `deny = don't propagate`. Do not
  revert the container. Drift is fine if contained; it's only a problem
  when promoted. Destructive `deny + revert` is explicitly out of scope
  until demand surfaces.
- **Policy engine scope.** Trust allowlist needs to cover package
  managers (npm, apt, pip, cargo), MCP sources, and skill patterns.
  Start with coarse-grained: match by kind + glob, no complex predicates.

### Phase 3 (forward-looking)

- **Multi-holder Open.** Natural pair-programming model; daemon needs
  session-level conflict detection (two humans typing into the same
  agent).
- **Role RBAC.** Requires the user model deferred from Phase 1.
- **Role registry.** Inter-fleet sharing = commercial-tier feature; OSS
  ships local publish only.

## Consequences

**Makes easier:**

- A coherent mental model for self-improving vs reproducible agents.
  "Is this agent mutable?" has a one-word answer.
- Team coordination on role iteration. One Open per role = one
  iterator = no merge conflicts.
- Spot tier absorbs the existing ephemeral-agent story from ADR 0004
  cleanly.
- Third-party runtimes (OpenClaw, Hermes) slot in as Open-tier citizens
  without forcing them to fight their self-updating nature. See
  forthcoming ADR on runtime adapters.
- Promotion at three granularities (skill → capability → image) gives
  users the right tool for the job.

**Makes harder:**

- Image version management (storage, rollback, retention) becomes a
  first-class concern.
- Daemon must enforce single-Open invariant and track holders.
- Restart coordination when Pinned image changes adds operational
  complexity vs the current always-running model.
- Role resource kind adds another fleet YAML shape to learn alongside
  Agent and Human (ADR 0017).

## Implementation notes

**Phase 1 (estimated scope):**

- New resource kind: `Role` in `packages/daemon/src/fleet/parser.ts` and
  validator.
- New commands in daemon API: `POST /v1/roles/:name/open`,
  `POST /v1/roles/:name/publish`, `POST /v1/roles/:name/release`.
- CLI wrappers: `zund role <name> open|publish|release`.
- Image publish: wrap `POST /1.0/containers/<name>/publish` Incus call.
- Image GC: extend `incus/image.ts` with retention enforcement.
- State: track Open holder per role in daemon in-memory state, persisted
  alongside fleet state.
- Rolling restart: extend `fleet/executor.ts` to diff image versions on
  apply and sequence Pinned restarts.

**Naming convention:**

- Open container: `zund-role-<role>-open`
- Pinned container: `zund-role-<role>-pinned-<n>`
- Spot container: `zund-role-<role>-spot-<uuid>` (auto-deleted on stop)
- Published image: `zund/<role>:<tag>` (tag = auto-increment integer)

**Migration from current model:**

- Current agents in `fleet/agents/*.yaml` continue to work as Pinned
  agents with implicit singleton role. A migration step (Phase 1 tail)
  converts them to explicit `fleet/roles/*.yaml` when the operator is
  ready.
- Ephemeral agents (ADR 0004) become Spot in new terminology; wire-up
  code already exists.

**Relationship to ADR 0017 (humans as fleet members):**

- Humans have no Open/Pinned/Spot — they're an orthogonal runtime kind.
- But the dispatcher (Phase 3) may treat human approval as a
  prerequisite for Open claim or publish, depending on role policy.

## Next steps

- Review this ADR; accept or amend before any implementation begins.
- Spike: single-role end-to-end Phase 1 flow on the `researcher`
  example in `samples/`. Prove the `incus publish` → Pinned restart
  loop before generalizing.
- Defer Phase 2 ADRs until Phase 1 ships and real self-improvement
  behavior is observed — the proposal taxonomy should be informed by
  what actually shows up in Open containers in practice.
- Update `docs/reference/architecture.md` component map to reflect role
  resource and tier tracking once Phase 1 lands.
