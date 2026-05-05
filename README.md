<div align="center">

# DocOps

**Typed project-state substrate for LLM-first development**

[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/logicwind/docops)](https://goreportcard.com/report/github.com/logicwind/docops)
[![GitHub Release](https://img.shields.io/github/v/release/logicwind/DocOps?include_prereleases)](https://github.com/logicwind/DocOps/releases)

[Install](#install) · [Quickstart](#quickstart) · [How it works](#how-it-works) · [CLI reference](#cli-reference) · [Developing](#developing-on-docops-itself)

</div>

---

> **DocOps is in active development.** The core CLI commands (`init`, `validate`, `index`, `state`, `audit`, `new`, `schema`, `refresh`, `get`, `list`, `graph`, `next`, `search`, `upgrade`) are shipped and stable. Additional commands (`serve`, `html`, `amend`, `review`, `status`) are on the roadmap — see [`docs/STATE.md`](docs/STATE.md) for what's live and what's coming.

## Why DocOps?

Coding agents (Claude Code, Cursor, Aider, Codex, Copilot CLI, Windsurf) land in a repo and immediately ask: *what are we building, what was decided, and what's next?* When that context lives in Slack threads, Jira tickets, or tribal knowledge, agents — and humans — guess.

DocOps puts three document types in `docs/`, typed with YAML frontmatter, linked with explicit edges, and validated by a CLI:

| Type | Folder | Holds | Example |
|---|---|---|---|
| **Context** | `docs/context/` | Stakeholder inputs — PRDs, memos, research, interview notes | Product brief, user research |
| **Decision** | `docs/decisions/` | Architecture and process decisions (ADR format) | "Use SQLite for local state" |
| **Task** | `docs/tasks/` | Units of work that cite ≥1 decision or context | "Wire up SQLite per ADR-0012" |

The key invariant: **every task must cite at least one decision or context document.** `docops validate` enforces this. This is the alignment contract — it prevents drift between "what we're building" and "what we said we'd build."

No other tool in the space enforces this.

## How it works

```
docs/
  context/CTX-001-vision.md      ← stakeholder intent
  decisions/ADR-0001-pick-db.md   ← how we chose
  tasks/TP-003-wire-sqlite.md    ← what to build (cites ADR-0001)
  .index.json                    ← computed graph (don't edit)
  STATE.md                       ← generated snapshot (don't edit)
docops.yaml                      ← project config (context types, gap thresholds)
```

1. **`docops init`** scaffolds the folder structure, schemas, agent skills, and `AGENTS.md` into any git repo.
2. **`docops new`** creates documents with auto-allocated IDs and validated frontmatter.
3. **`docops validate`** checks schema and graph invariants (citations resolve, no dangling refs, task alignment rule).
4. **`docops index`** builds the enriched graph; **`docops state`** generates a human-readable snapshot.
5. **`docops audit`** finds structural gaps: accepted decisions with no tasks, stalled tasks, stale reviews.
6. **Agents read `STATE.md` → pick a task → read its cited ADRs → code → update status → `docops refresh`.**

The CLI is the query and mutation API. Every read command supports `--json` for scripting. See [ADR-0018](docs/decisions/ADR-0018-cli-as-query-layer.md) for the design rationale.

## Install

### macOS / Linux (Homebrew)

```sh
brew install logicwind/tap/docops
```

### Windows (Scoop)

```sh
scoop bucket add logicwind https://github.com/logicwind/scoop-bucket
scoop install docops
```

### Beta channel

Opt-in prereleases (`vX.Y.Z-beta.N`, `-alpha.N`, `-rc.N`) ship to a parallel formula / manifest in the same tap and bucket. Stable installs are unaffected.

```sh
brew install logicwind/tap/docops@beta   # macOS / Linux
scoop install docops-beta                # Windows (after `scoop bucket add logicwind ...`)
```

See [ADR-0032](docs/decisions/ADR-0032-beta-release-channel-via-beta-tap-formula.md) for the channel design.

### Direct download

Grab the archive for your platform from [GitHub Releases](https://github.com/logicwind/DocOps/releases), extract, and put `docops` on your PATH.

### Docker and npm

Docker image (GHCR) and npm convenience shims (`@docops/cli`) are planned for a future release. See [ADR-0012](docs/decisions/ADR-0012-language-agnostic-distribution.md) for the distribution rationale.

## Quickstart

From the root of any git repo:

```sh
docops init                                        # scaffold everything
```

DocOps detects whether the repo is **greenfield** (empty) or
**brownfield** (existing code) and points you at the right next move.

- **Greenfield:** `docops new ctx "Product vision" --type brief` to capture
  the brief, then `/docops:plan` to drive the first ADR + tasks.
- **Brownfield:** run `/docops:onboard` — the agent scans the codebase,
  asks 3–5 clarifying questions, and drafts CTX-001 + 1–3 ADRs from
  load-bearing decisions visible in the code.

After that, the loop is the same:

```sh
docops new adr "Pick a database"                    # capture a decision
docops new task "Wire up SQLite" --requires ADR-0001 # task citing the decision
docops refresh                                      # validate + index + state in one pass
docops audit                                        # find structural gaps
```

Every mutating command ends with a `→ Next:` block of suggested
follow-ups. Add `--quiet` to suppress, or `--json` for programmatic
output (which carries the suggestions in a `next_steps` array).

`docops init` flags: `--dry-run` (preview), `--force` (re-sync drifted files), `--no-skills` (skip agent skill scaffolding), `--json` (structured output).

### Upgrading

```sh
brew upgrade docops              # or scoop update docops
docops upgrade                   # sync skills, schemas, AGENTS.md
docops upgrade --dry-run         # preview first
```

`docops upgrade` only touches DocOps-owned scaffolding. To also rewrite `docops.yaml` or reinstall the pre-commit hook, use `--config` or `--hook`. Run `docops update-check` to see if a new version is available.

## CLI reference

| Command | What it does |
|---|---|
| `docops init` | Scaffold DocOps into a repo (idempotent) |
| `docops upgrade` | Refresh DocOps-owned files in an existing project |
| `docops validate` | Schema + graph invariants; exits non-zero on errors |
| `docops index` | Build `docs/.index.json` (enriched graph) |
| `docops state` | Regenerate `docs/STATE.md` (counts, active work, gaps) |
| `docops audit` | Structural gap punch list |
| `docops refresh` | validate + index + state in one pass |
| `docops schema` | (Re)write JSON Schemas from `docops.yaml` |
| `docops new` | Scaffold a new CTX, ADR, or Task document |
| `docops get <ID>` | Look up one document by ID |
| `docops list` | List docs with filters (`--kind`, `--status`, `--tag`) |
| `docops graph <ID>` | Typed edge graph from a starting doc |
| `docops next` | Recommend the next task to work on |
| `docops search <query>` | Substring/regex search over title, tags, body |
| `docops html` | Emit a browsable HTML viewer to `docs/.html/` |
| `docops serve` | Localhost web viewer on `:8484` — sidebar, graph, live |

All commands support `--json` for structured output. Run `docops <command> --help` for details.

### HTML viewer

`docops serve --open` spins up a localhost web UI for the current repo: sidebar by kind (CTX / ADR / TP), frontmatter + rendered markdown on the right, and an interactive graph tab. Hover a node to focus its neighborhood, single-click to pin, double-click to open the doc. Works on any modern browser; no install, no framework — the SPA pulls Tailwind / `marked` / `cytoscape` from jsDelivr on first load.

## Editor integration

`docops init` (and `docops schema`) write JSON Schemas to `docs/.docops/schema/`. Install [`redhat.vscode-yaml`](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml) and add to your `.vscode/settings.json`:

```json
"yaml.schemas": {
  "./docs/.docops/schema/context.schema.json":  "docs/context/*.md",
  "./docs/.docops/schema/decision.schema.json": "docs/decisions/*.md",
  "./docs/.docops/schema/task.schema.json":     "docs/tasks/*.md"
}
```

## What DocOps is not

DocOps is a **substrate**, not a framework. It provides typed state — not workflow, not orchestration, not personas, and not code generation. See [ADR-0014](docs/decisions/ADR-0014-positioning-substrate-not-harness.md) for the full scope boundaries.

- Not a phase orchestrator (that's GSD's domain).
- Not a role/persona system (that's GStack's domain).
- Not a code generator or execution harness.
- Not a hosted dashboard or issue tracker.

## Documentation

- **[`docs/STATE.md`](docs/STATE.md)** — current project state (auto-generated)
- **[`docs/context/`](docs/context/)** — stakeholder inputs and research
- **[`docs/decisions/`](docs/decisions/)** — architecture decisions (ADRs)
- **[`docs/tasks/`](docs/tasks/)** — work items with citation requirements
- **[`AGENTS.md`](AGENTS.md)** — orientation for coding agents working on DocOps itself
- **[`CHANGELOG.md`](CHANGELOG.md)** — release history

## Contributing

Issues, feature requests, and pull requests are welcome on [GitHub](https://github.com/logicwind/DocOps/issues). This repo dog-foods DocOps: all changes go through the same `validate` → `index` → `state` cycle.

See [`AGENTS.md`](AGENTS.md) for the orientation guide if you're an agent, and the Makefile targets for the local development workflow:

```sh
make tidy     # go mod tidy
make build    # builds bin/docops
make test     # go test -race ./...
make lint     # go vet ./...
```

## Developing on DocOps itself

This repository is the DocOps **source**, and it dog-foods its own convention. The root `AGENTS.md` separates the "meta" layer (this repo's own project management) from the "product" layer (what `docops init` emits into user repos). See [ADR-0016](docs/decisions/ADR-0016-meta-vs-product-separation.md).

### Release

Two channels: **stable** for everyone, **beta** for opt-in testers and your own dogfooding. Full runbook in [`CTX-005`](docs/context/CTX-005-release-runbook-stable-and-beta-channels.md).

```sh
# fast loop — tweak source, test in another project on this machine
make install

# dogfood — cut a prerelease from any branch
make beta VERSION=0.6.1-beta.1

# promote — once the beta has held up, cut stable from clean main
make release VERSION=0.6.1
```

Tag pushes trigger `.github/workflows/release.yml`, which runs goreleaser to build the matrix, attach artifacts to the GitHub Release, and update the Homebrew/Scoop stubs. Prerelease tags route to `docops@beta` / `docops-beta` only; stable tags route to `docops` / `docops`.

Dry-run: `make release VERSION=0.4.1 DRY_RUN=1`. Local snapshot (no tag, no push): `make release-snapshot`.

## License

MIT © [Logicwind Technologies Pvt Ltd](https://logicwind.com) — see [`LICENSE`](LICENSE).

DocOps is built and maintained by [Logicwind](https://logicwind.com).