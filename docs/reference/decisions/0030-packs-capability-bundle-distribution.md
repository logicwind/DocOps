---
id: "0030"
title: "Packs — capability bundle distribution unit"
date: 2026-04-18
status: draft
implementation: not-started
supersedes: []
superseded_by: null
related: ["0012", "0020", "0023", "0025", "0027", "0028", "0029"]
tags: [packs, plugins, capabilities, distribution, skills, mcp]
---

# 0030 · Packs — capability bundle distribution unit

Date: 2026-04-18
Status: draft
Related: ADR 0012 (secrets — packs declare `secrets_required`),
ADR 0020 (plugin architecture — packs compose plugin-kind bindings),
ADR 0023 (task queue — apply-time failures surface as pending),
ADR 0025 (docs store — some pack skills reference docs),
ADR 0027 (Pi baseline extensions — packs extend, not replace, the
core seven), ADR 0028 (fleet_capabilities + runtime_config — packs
contribute to both), ADR 0029 (mcporter sidecar — packs contribute
`mcp_servers:`).

## Context

Zund's product wedge is the inverse of Hermes/OpenClaw. Those
runtimes ship monolith agents with 60–118 skills each, loaded into a
single process. Zund is lightweight Pi agents, specialized,
coordinated via fleet primitives, out-performing a monolith through
parallelism + specialization.

The tension: "lightweight" can feel "stripped down" without curated
capability bundles. A user running `zund apply` on a fresh install
gets Pi + seven extensions (ADR 0027) + whatever they declared in
YAML. That's genuinely enough to build real fleets, but the
out-of-box comparison against Hermes ("here are 60 skills") looks
bad on paper.

The fix is **curated bundles that are opt-in, not always-on**. The
user keeps their small surface by default but can add bundles of
related capabilities with a single line of YAML.

Three-tier capability model (ADR 0028) gives us the seam: packs are
**distribution units** that contribute to `fleet_capabilities:`
(rarely), `runtime_config:` (sometimes), and `mcp_servers:` (often),
plus shipping skill markdown files agents can load on demand. Packs
are not a fifth tier — they are a way to bundle things that live in
the existing tiers.

## Decision

Introduce **Packs** as the opt-in capability distribution unit. A
pack bundles:

- Zero or more **skills** (SKILL.md files, loaded on demand per the
  skill invariant in ADR 0028).
- Zero or more **MCP servers** (contributed to the fleet sidecar per
  ADR 0029).
- A list of **secrets_required** (validated against the secrets
  plugin at apply time).
- Optional **runtime_constraints** (minimum runtime version).

Fleet YAML declares which packs are enabled per agent. `zund apply`
resolves packs into concrete contributions across the three tiers.

### 1. Pack manifest shape (`pack.yaml`)

```yaml
name: github-workflow
version: 1.0.0
description: "Code review, PR workflow, issue triage via GitHub's official MCP"

skills:
  - path: skills/github-pr-workflow/SKILL.md
  - path: skills/github-code-review/SKILL.md
  - path: skills/github-issue-triage/SKILL.md

mcp_servers:
  - name: github
    transport: stdio
    command: npx -y @modelcontextprotocol/server-github
    env:
      GITHUB_TOKEN: ref://secrets.GITHUB_TOKEN

secrets_required:
  - GITHUB_TOKEN

runtime_constraints:
  # optional — if absent, pack works for any runtime that supports
  # the fleet_capabilities it depends on.
  min_runtime:
    pi: ">=0.5.0"
    # or:
    # hermes: ">=0.10.0"
    # openclaw: ">=0.3.0"

fleet_capabilities:
  # optional — if set, agents enabling this pack must also declare
  # these fleet_capabilities. Typically [mcp] for packs that ship
  # MCP servers; rarely more.
  - mcp
```

Every field except `name`, `version`, and `description` is optional.
A pack consisting of nothing but skill markdown (no MCP, no secrets)
is valid — it is the lightest possible pack.

### 2. Pack directory structure

```
packs/
  github-workflow/
    pack.yaml
    skills/
      github-pr-workflow/
        SKILL.md
        assets/                ← optional images, templates, etc.
      github-code-review/
        SKILL.md
      github-issue-triage/
        SKILL.md
```

Skills are copied **by value** into agent workspaces at apply time
(next section). The pack directory is the source of truth; agent
copies are artifacts.

### 3. Fleet YAML — `packs:` block

```yaml
# fleet/researcher.yaml
name: researcher
runtime: pi
fleet_capabilities: [memory, docs, fleet-status, task-delegate, mcp]
packs:
  - research-primitives
  - github-workflow
  - productivity-gws
```

Packs are enabled per-agent. Two agents in the same fleet can enable
different packs; their MCP contributions union into the fleet's
sidecar (ADR 0029 §7), but their skill workspaces remain agent-local.

### 4. `zund apply` pack resolution

Step-by-step, per agent:

1. **Load pack manifests** for each pack name in `packs:`.
   Resolution order: user-local `fleet/packs/`, then workspace
   `packages/packs/`, then installed contrib packages
   (`node_modules/@zund/pack-*`).
2. **Check `runtime_constraints.min_runtime`** against the agent's
   runtime + version. Violation → apply fails with a clear error.
3. **Validate `secrets_required`**. For each pack, every listed
   secret must exist in the active secrets plugin's store. Missing
   → apply fails with "pending: secret X required by pack Y" per
   ADR 0023.
4. **Union MCP contributions.** Pack `mcp_servers:` entries merge
   into the fleet-level `mcp_servers:` block consumed by the
   mcporter sidecar (ADR 0029). Name collisions (same MCP server
   name from two packs) fail apply — operator must resolve
   explicitly.
5. **Copy skill files.** Each `skills: [{ path }]` entry copies the
   skill directory into the agent's skills workspace. For Pi, this
   is where `load_skill` looks. For Hermes/OpenClaw, the runtime
   plugin's `bridgeFor()` equivalent determines the destination.
6. **Emit events.** One `data-z:fleet:pack-loaded` per pack per
   agent (new event per this ADR; extends ADR 0022 catalog).

On any failure at steps 2–4, apply aborts for that agent and the
agent enters the `pending` state per ADR 0023 (not failed — the
operator can fix and retry).

### 5. Distribution tiers

**Bundled:** live in `packages/packs/` in the zund monorepo. Shipped
with every zund install. Curated set below (§6).

**Contrib:** npm packages named `@zund/pack-<name>`. Installed via
`zund pack install <name>`, which adds the dependency to the
workspace and makes the pack available. No code-path execution at
install — only manifest + skill + config files.

**User-local:** in a fleet's own repo under `fleet/packs/<name>/`,
git-tracked. Used for private team workflows that should not be
published. Resolution picks these up ahead of workspace or contrib
packs, enabling local override.

### 6. v1 curated pack set (bundled)

Six packs ship with v1. Each is described by: what problem it solves,
what's inside, what secrets it needs, and a short demo path.

#### `research-primitives`

- **Problem:** baseline research flow (search, read, summarize)
  beyond Pi's built-in web-search extension.
- **Contents:**
  - Skills: `deep-web-research`, `arxiv-paper-workflow`,
    `long-document-summarization`.
  - MCP servers: arxiv MCP (for paper search/fetch).
- **Secrets:** `TAVILY_API_KEY` (reuses the Pi web-search key,
  nothing pack-specific).
- **Demo:** "Research the state of small-context LLM fine-tuning,
  cite three papers."

#### `github-workflow`

- **Problem:** code-centric workflows (PR review, issue triage,
  repo exploration) via the official GitHub MCP server.
- **Contents:**
  - Skills: `github-pr-workflow`, `github-code-review`,
    `github-issue-triage`.
  - MCP servers: `@modelcontextprotocol/server-github`.
- **Secrets:** `GITHUB_TOKEN`.
- **Demo:** "Review the open PRs on `zund/zund` and post summaries."

#### `productivity-gws`

- **Problem:** Gmail / Calendar / Drive access without each agent
  reinventing OAuth.
- **Contents:**
  - Skills: `gmail-triage`, `calendar-scheduling`,
    `drive-doc-workflow`.
  - MCP servers: community Google Workspace MCP server
    (uvx-installed).
- **Secrets:** Google OAuth — handled via the sidecar's
  `~/.mcporter/gws/` persistent volume (ADR 0029 §5). First-run
  wizard: `zund auth productivity-gws` walks OAuth consent.
- **Demo:** "Summarize today's unread mail, schedule a follow-up for
  the hot thread."

#### `team-ops`

- **Problem:** operational workflows across Linear / Notion / Slack.
- **Contents:**
  - Skills: `linear-issue-triage`, `notion-doc-sync`,
    `slack-standup-digest`.
  - MCP servers: Linear (remote HTTP), Notion (stdio), Slack
    (stdio).
- **Secrets:** `LINEAR_TOKEN`, `NOTION_TOKEN`, `SLACK_BOT_TOKEN`.
- **Demo:** "Summarize the week's Linear progress and post to #eng."

#### `docs-io`

- **Problem:** handling user-uploaded PDFs, OCR, long-document
  processing at higher quality than `parser-native`.
- **Contents:**
  - Skills: `pdf-question-answering`, `document-extraction`,
    `multi-doc-synthesis`.
  - No MCP servers; this pack composes with ADR 0025 parser
    plugins (e.g., `parser-liteparse`, `parser-mistral-ocr`).
- **Secrets:** optional `MISTRAL_API_KEY` (only if operator chose
  the hosted OCR parser).
- **Demo:** "Index these 200 PDFs; answer questions with page
  citations."

#### `browser-automation`

- **Problem:** web automation (scraping, form-filling, screenshot
  QA) without wiring Playwright per-agent.
- **Contents:**
  - Skills: `browser-navigate-login`, `browser-extract-structured`,
    `browser-visual-diff`.
  - MCP servers: `@modelcontextprotocol/server-playwright`.
- **Secrets:** none (unless the site needs creds, in which case
  `ref://secrets.SITE_X_COOKIE` passes through).
- **Demo:** "Scrape the latest prices from this e-commerce site,
  diff against yesterday."

### 7. Explicitly excluded from v1

- Apple suite (Reminders, Notes, Messages, Shortcuts) — Zund is
  container-first; Apple-native integration requires macOS host
  access that cuts against the fleet model.
- Blockchain / crypto tooling.
- Gaming integrations.
- Red-team / offensive-security skills.
- Niche MLops (specific to single vendors without generalizable
  workflow value at v1 scale).

These are not rejected forever. Any contributor can publish a
contrib pack (`@zund/pack-*` on npm) at any time. The bundled set is
curated for clarity, not gatekept by principle.

### 8. Promotion path contrib → bundled

A contrib pack can be promoted to bundled via PR. Criteria:

- Stable for at least one minor Zund release cycle.
- Works reliably across at least two runtimes (if it uses
  `fleet_capabilities:` that multiple runtimes support) or
  documents why it's runtime-scoped.
- Has a credible OAuth-less path **or** a clear `zund auth` wizard
  entry.
- Ships with at least one reference fleet YAML demonstrating usage.

Promotion is lightweight — the bar is demonstrated usage, not a
bureaucratic review.

### 9. Pack versioning

Packs carry `version:` in their manifest. `zund apply` captures the
pack version per agent at apply time so the deployed fleet is
reproducible. `zund pack install <name>@<version>` pins contrib pack
versions. Breaking changes in a pack (removing a skill, changing a
required secret) require a major-version bump.

## Challenges and open questions

### Pack collisions

Two enabled packs declaring the same MCP server name (e.g., two
Slack packs both named `slack`) fail apply. Operators resolve by
disabling one. This is noisy but correct — silent override would be
worse.

### Skill name collisions across packs

Unlike MCP server names, skill names can collide (two packs ship a
`code-review` skill). Resolution: scope skills by pack name at copy
time, so the loaded skill is `github-workflow/code-review.md`, not
`code-review.md`. The skill's `load_skill` invocation names the
scoped path.

### Pack trust model

Packs are content + config. They don't execute code at install time.
But MCP servers they declare *do* execute (via `npx -y` or
`uvx run`). The trust boundary is the MCP server, not the pack.
Operators accepting a pack are accepting its MCP server
dependencies. Bundled packs vet this; contrib packs are caveat
emptor, with the pack manifest listing every `command` up-front so
the operator can audit.

### OAuth-heavy packs (GWS, Linear, Notion, Slack)

Six OAuth flows is a lot of setup friction for a first run. The
`zund auth <pack>` wizard (future CLI) walks consent per pack. Until
that wizard exists, operators do the OAuth dance once against the
underlying MCP server's docs. Pack manifests include an `auth:`
section pointing to per-server instructions.

### Pack dependencies on other packs

Packs do not depend on other packs in v1. Every pack is
self-contained. If a workflow emerges where `pack-a` wants to reuse
skills from `pack-b`, the operator enables both packs; skills live
in separate namespaces. If real dependency graphs emerge, add a
`depends:` field in v2 — not before.

### Disabling a pack mid-fleet

Removing a pack from an agent's `packs:` list at apply time:

- Removes skill files from the agent workspace.
- Removes MCP server contributions from the sidecar config
  (sidecar restarts if config changed, per ADR 0029).
- Does **not** revoke OAuth tokens (those stay in the sidecar
  config volume for future re-enable).

Operators wanting to revoke tokens must do so at the source (the
third-party's OAuth console).

### Inter-pack secret reuse

Two packs both requiring `GITHUB_TOKEN` share the same secret. This
works trivially — `ref://secrets.GITHUB_TOKEN` resolves to one
value. The `secrets_required` union is the actual required set.
Document.

## Consequences

**Makes easier:**

- **Out-of-box story gets competitive.** `zund apply` with three
  bundled packs looks credible against Hermes's 60-skill monolith.
- **Ecosystem participation is clean.** Any team can publish a
  contrib pack on npm. The bundled set stays curated; the ecosystem
  grows without Zund blessing every addition.
- **Fleet YAML stays small.** One line (`packs: [...]`) pulls in a
  bundle instead of declaring every skill + MCP server + secret.
- **Distribution matches user mental model.** "I want GitHub
  workflows" is a pack, not seven lines of config.
- **Composes cleanly with ADR 0028/0029.** Packs contribute to
  existing tiers; no new plumbing for runtimes to support.

**Makes harder:**

- **Pack authoring guide becomes load-bearing.** Quality of
  third-party packs determines user experience. Needs docs,
  examples, maybe a validator CLI.
- **Versioning discipline.** Packs need semver, and zund apply
  needs to record what was applied to stay reproducible.
- **Trust model asymmetry.** Bundled packs are vetted; contrib packs
  are not. Operators need to understand the difference.
- **Pack + runtime + service plugin triple-matrix.** Debugging "my
  pack's MCP server isn't working" requires knowing which pack,
  which runtime bridge, which mcporter version. Good error messages
  and `zund fleet status` surfacing mitigate.

## Relationship to existing ADRs

| ADR | Relationship |
|-----|-------------|
| 0012 | `secrets_required` validated against the active secrets plugin at apply time. |
| 0020 | Packs are not plugins; they compose plugin-kind bindings. Pack-loader lives in daemon, not as a plugin kind. |
| 0022 | `data-z:fleet:pack-loaded` (one per pack per agent) is new; `data-z:fleet:pack-failed` surfaces apply failures. Extends catalog. |
| 0023 | Missing `secrets_required` → agent enters pending state with `pendingReason: "secret X required by pack Y"`. Fits the existing pending flow. |
| 0025 | `docs-io` pack composes with the docs plugin's parser kind. Packs do not themselves define docs-tier plugins; they consume them. |
| 0027 | Packs extend the surface; they do not replace Pi's seven core extensions. Packs are opt-in; the seven are on by default. |
| 0028 | Packs can declare `fleet_capabilities:` required (usually `[mcp]` if the pack ships MCP servers). Packs can contribute `runtime_config:` fragments (rare, but allowed). |
| 0029 | Pack `mcp_servers:` entries union into the fleet mcporter sidecar. Apply-time reconciliation handles the merge + restart. |

## Implementation notes

**New daemon module:**

```
packages/daemon/src/packs/
  manifest.ts          ← pack.yaml schema + loader
  resolver.ts          ← resolve name → manifest across the three tiers
  apply.ts             ← pack resolution during zund apply
  events.ts            ← pack-loaded / pack-failed wire events
```

**New package root:**

```
packages/packs/                    ← bundled packs, workspace packages
  research-primitives/
    pack.yaml
    skills/
      deep-web-research/SKILL.md
      arxiv-paper-workflow/SKILL.md
      long-document-summarization/SKILL.md
  github-workflow/
    pack.yaml
    skills/
      github-pr-workflow/SKILL.md
      github-code-review/SKILL.md
      github-issue-triage/SKILL.md
  productivity-gws/
  team-ops/
  docs-io/
  browser-automation/
```

**Fleet parser changes:**

```
packages/daemon/src/fleet/parser.ts
  - accept top-level packs: [string] on agent
packages/daemon/src/fleet/types.ts
  - AgentResource gains packs?: string[]
```

**Apply path addition:**

Run after secrets resolution, before container start:

1. For each agent, resolve its `packs:` list → manifests.
2. Validate `runtime_constraints` and `secrets_required`.
3. Union `mcp_servers:` contributions into the fleet-level set (feeds
   ADR 0029 sidecar reconciliation).
4. Copy skill files into agent workspace (namespaced by pack).
5. Emit `data-z:fleet:pack-loaded` events.

**CLI verbs:**

```
zund pack list                          — show enabled packs per agent
zund pack install <name>[@<ver>]        — contrib pack install
zund pack uninstall <name>              — remove contrib pack
zund pack info <name>                   — show manifest contents
zund pack search <query>                — search npm for @zund/pack-*
zund auth <pack>                        — pack-specific auth wizard (future)
```

**No changes to:** the runtime contract (packs are apply-time
concerns; runtimes see the resolved output), the task queue, the
docs store, the wire protocol base (events are additive).

## Next steps

- Write `pack.yaml` schema validator (TypeBox) and author guide.
- Spike `research-primitives` end-to-end as the v0.5 reference pack.
- Design the `zund auth <pack>` wizard against one OAuth pack
  (productivity-gws) to validate the UX before other OAuth packs
  ship.
- Slot pack-loader implementation into `roadmap/current.md` after
  ADR 0029 sidecar lands (packs without a sidecar can't contribute
  MCP servers, so sidecar is the prerequisite).
