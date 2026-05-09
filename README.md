<div align="center">

# DocOps

**Typed project-state substrate for LLM-first development**

[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/logicwind/docops)](https://goreportcard.com/report/github.com/logicwind/docops)
[![GitHub Release](https://img.shields.io/github/v/release/logicwind/DocOps?include_prereleases)](https://github.com/logicwind/DocOps/releases)

[Core concepts](#core-concepts) · [Slash commands](#how-youll-use-it-slash-commands-first) · [Install](#install) · [Quickstart](#quickstart) · [Daily workflow](#daily-workflow) · [CLI reference](#cli-reference-the-engine)

</div>

---

> **DocOps is in active development.** The primary surface is **slash commands** (`/docops:init`, `/docops:onboard`, `/docops:plan`, `/docops:progress`, `/docops:next`, `/docops:do`) inside agent harnesses (Claude Code, Cursor, Codex). The CLI is the engine underneath; for day-to-day use, the only command you'll typically reach for directly is **`docops serve`** (the HTML viewer). See [`docs/STATE.md`](docs/STATE.md) for what's live and what's coming.

## Why DocOps?

Coding agents (Claude Code, Cursor, Aider, Codex, Copilot CLI, Windsurf) land in a repo and immediately ask: *what are we building, what was decided, and what's next?* When that context lives in Slack threads, Jira tickets, or tribal knowledge, agents — and humans — guess.

DocOps puts three document types in `docs/`, typed with YAML frontmatter, linked with explicit edges, and validated by a CLI. The key invariant: **every task must cite at least one decision or context document.** `docops validate` enforces this. This is the alignment contract — it prevents drift between "what we're building" and "what we said we'd build."

No other tool in the space enforces this.

## Core concepts

Three document types, three roles. Read this once and the rest of the docs make sense.

| Prefix | Type | Folder | Answers | Example |
|---|---|---|---|---|
| **`CTX-`** | Context | `docs/context/` | *Why are we doing this?* | `CTX-001-product-vision.md` (a PRD, memo, research note, or brief) |
| **`ADR-`** | Decision | `docs/decisions/` | *What did we choose, and why this over the alternatives?* | `ADR-0012-pick-sqlite.md` |
| **`TP-`** | Task | `docs/tasks/` | *What concrete work falls out of that decision?* | `TP-003-wire-sqlite.md` (cites `ADR-0012`) |

```mermaid
flowchart LR
    CTX["<b>CTX-001</b><br/>Context<br/><i>brief · PRD · memo · research</i>"]
    ADR["<b>ADR-0012</b><br/>Decision<br/><i>chosen approach + tradeoffs</i>"]
    TP["<b>TP-003</b><br/>Task<br/><i>unit of work</i>"]

    CTX -- "informs" --> ADR
    ADR -- "decides scope of" --> TP
    CTX -. "(may directly require)" .-> TP

    classDef ctx fill:#e7f0ff,stroke:#3b6fd0,color:#0a2c66
    classDef adr fill:#fff4e0,stroke:#c08a00,color:#5a3d00
    classDef tp  fill:#e6f7ec,stroke:#2f9e5a,color:#0d3d22
    class CTX ctx
    class ADR adr
    class TP tp
```

**Reading order for a new contributor (or agent):**

1. **CTX** for the *why* — start with `CTX-001` if it exists; it's the product brief.
2. **ADR** for the *how* it was decided — frontmatter `status:` tells you if it's `draft`, `accepted`, or `superseded`.
3. **TP** for the *what* you can pick up — every task lists its `requires:` (citations) and `depends_on:` (other tasks).

**The alignment contract:** a task with no `requires:` field fails validation. You cannot ship work that isn't traceable to a decision or stakeholder input.

## How you'll use it (slash commands first)

DocOps is designed to be driven from inside your agent (Claude Code, Cursor, Codex). Six slash commands cover every moment in the loop — you almost never type `docops <verb>` by hand.

| Slash command | When to reach for it | What it does |
|---|---|---|
| **`/docops:init`** | First time in a repo | Scaffolds `docs/`, schemas, `AGENTS.md`/`CLAUDE.md` blocks, pre-commit hook |
| **`/docops:onboard`** | Brownfield repo with existing code | Scans the codebase, asks 3–5 questions, drafts `CTX-001` + 1–3 ADRs from load-bearing decisions |
| **`/docops:plan`** | You have a CTX (PRD, memo, research) | Drafts one ADR + tasks that cite it (human confirms before write) |
| **`/docops:progress`** | "Where are we?" | Reads `STATE.md`, runs audit, names the next action |
| **`/docops:next`** | "What should I pick up?" | Recommends one task using priority, status, dependencies |
| **`/docops:do`** | You know the intent, not the verb | Routes natural language to the right skill or CLI call |

```mermaid
flowchart LR
    subgraph You["You / your agent"]
        direction TB
        U1["/docops:init"]
        U2["/docops:onboard"]
        U3["/docops:plan"]
        U4["/docops:progress"]
        U5["/docops:next"]
        U6["/docops:do"]
    end

    subgraph Engine["docops CLI (engine)"]
        direction TB
        E1["init · validate · index<br/>state · audit · refresh"]
        E2["new · get · list · graph<br/>next · search"]
    end

    subgraph Repo["docs/ (typed files)"]
        direction TB
        R1["CTX-*.md · ADR-*.md · TP-*.md"]
        R2["STATE.md · .index.json<br/><i>(generated)</i>"]
    end

    You -->|invoke| Engine
    Engine -->|reads + writes| Repo
    Repo -->|"docops serve · HTML viewer"| BROWSE(["browse in browser"])

    classDef slash fill:#e7f0ff,stroke:#3b6fd0,color:#0a2c66
    classDef engine fill:#fff4e0,stroke:#c08a00,color:#5a3d00
    classDef repo fill:#e6f7ec,stroke:#2f9e5a,color:#0d3d22
    class U1,U2,U3,U4,U5,U6 slash
    class E1,E2 engine
    class R1,R2 repo
```

The slash commands are **opinionated workflows** that wrap the CLI. The CLI is still there when you need scripting, automation, or a moment that doesn't fit one of the six skills — but reach for slash first.

The one CLI command you *will* use directly: **`docops serve`** — opens a localhost HTML viewer of your project state (sidebar, graph, live reload). Everything else is plumbing.

## Folder layout

```
docs/
  context/CTX-001-vision.md       ← stakeholder intent
  decisions/ADR-0001-pick-db.md   ← how we chose
  tasks/TP-003-wire-sqlite.md     ← what to build (cites ADR-0001)
  .index.json                     ← computed graph (don't edit)
  STATE.md                        ← generated snapshot (don't edit)
docops.yaml                       ← project config (context types, gap thresholds)
```

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

The whole quickstart is slash commands inside your agent. From the root of any git repo:

```
/docops:init             ← scaffold docs/, schemas, AGENTS.md, hooks
/docops:onboard          ← (brownfield) bootstrap CTX + ADRs from existing code
                           — or —
/docops:plan             ← (greenfield) draft an ADR + tasks from a CTX you wrote
```

That's it for setup. From here, the daily loop is just two slash commands:

```
/docops:progress         ← where are we? (STATE + audit + recommended next move)
/docops:next             ← which task should I pick up?
```

If you don't know which slash command fits, type `/docops:do <what you want>` and it'll route.

> **Bootstrap detail.** `/docops:init` requires the `docops` binary on your PATH (it shells out to `docops init`). Install it via the [Install](#install) section below — one `brew install` and you're done. After init, the slash commands are wired into `.claude/commands/docops/` and `.cursor/commands/docops/` for that repo.

## Onboarding flow

DocOps auto-detects greenfield vs brownfield. Either path is one slash command, and both converge on the same daily loop.

```mermaid
flowchart TD
    START(["/docops:init"]) --> DETECT{"Existing<br/>code in repo?"}

    DETECT -- "No (greenfield)" --> G1["You write CTX-001<br/><i>(product vision / brief)</i>"]
    G1 --> G2["/docops:plan"]
    G2 --> G3["Agent drafts<br/>1 ADR + first tasks<br/>citing CTX-001"]

    DETECT -- "Yes (brownfield)" --> B1["/docops:onboard"]
    B1 --> B2["Agent scans repo,<br/>asks 3–5 questions"]
    B2 --> B3["Drafts CTX-001<br/>+ 1–3 ADRs from<br/>load-bearing code"]

    G3 --> LOOP(["Daily loop:<br/>/docops:progress<br/>/docops:next"])
    B3 --> LOOP

    classDef start fill:#1f2937,stroke:#111,color:#fff
    classDef green fill:#e6f7ec,stroke:#2f9e5a,color:#0d3d22
    classDef brown fill:#fff4e0,stroke:#c08a00,color:#5a3d00
    classDef decision fill:#e7f0ff,stroke:#3b6fd0,color:#0a2c66
    class START,LOOP start
    class G1,G2,G3 green
    class B1,B2,B3 brown
    class DETECT decision
```

## Daily workflow

Two slash commands open the session; the rest is reading and coding.

```mermaid
flowchart LR
    A["/docops:progress<br/><i>state + audit + next move</i>"] --> B["/docops:next<br/><i>recommend one task</i>"]
    B --> C["Read the task's<br/><b>requires:</b> + <b>depends_on:</b><br/><i>(every CTX + ADR cited)</i>"]
    C --> D["Code the change"]
    D --> E["Mark task<br/>status: done"]
    E --> F["docops refresh<br/><i>(or your agent runs it)</i>"]
    F --> A

    classDef slash fill:#e7f0ff,stroke:#3b6fd0,color:#0a2c66
    classDef step fill:#fff,stroke:#444,color:#111
    classDef code fill:#e6f7ec,stroke:#2f9e5a,color:#0d3d22
    class A,B slash
    class C,E,F step
    class D code
```

The contract: **before coding, you must read every doc the task `requires:`.** That's how the agent (or a new contributor) inherits the *why*, not just the *what*.

To browse the project state in a real UI — sidebar, graph, frontmatter, rendered markdown — run **`docops serve --open`**. That's the one CLI command you'll actually type by hand.

## Upgrading

```sh
brew upgrade docops              # or scoop update docops
docops upgrade                   # sync skills, schemas, AGENTS.md
docops upgrade --dry-run         # preview first
```

`docops upgrade` only touches DocOps-owned scaffolding. To also rewrite `docops.yaml` or reinstall the pre-commit hook, use `--config` or `--hook`. Run `docops update-check` to see if a new version is available.

## CLI reference (the engine)

> **You probably don't need this section for daily use.** Slash commands cover the moments. The CLI is here for: bootstrap (`docops init` once per repo), browsing (`docops serve`), scripting/CI, and the rare manual edit. Every slash command is a thin wrapper over these verbs — read this when you want to script, not when you want to work.

Grouped by purpose. All commands support `--json` for structured output. Run `docops <command> --help` for details.

**Setup & maintenance**

| Command | What it does |
|---|---|
| `docops init` | Scaffold DocOps into a repo (idempotent) |
| `docops upgrade` | Refresh DocOps-owned files in an existing project |
| `docops update-check` | Probe upstream for a newer release (cached) |
| `docops schema` | (Re)write JSON Schemas from `docops.yaml` |

**Authoring**

| Command | What it does |
|---|---|
| `docops new ctx "title" --type <type>` | Create a context document (`CTX-NNN`) |
| `docops new adr "title"` | Create a decision record (`ADR-NNNN`) |
| `docops new task "title" --requires ADR-...,CTX-...` | Create a task (`TP-NNN`) — citation is required |

`docops new` accepts `--body TEXT`, `--body -` (stdin heredoc), or `--body-file PATH` to populate the body at creation. Use them to skip the stub-then-rewrite round-trip.

**Validation & generation**

| Command | What it does |
|---|---|
| `docops validate` | Schema + graph invariants; exits non-zero on errors |
| `docops index` | Build `docs/.index.json` (enriched graph) |
| `docops state` | Regenerate `docs/STATE.md` (counts, active work, gaps) |
| `docops audit` | Structural gap punch list |
| `docops refresh` | `validate` + `index` + `state` in one pass |

**Query**

| Command | What it does |
|---|---|
| `docops get <ID>` | Look up one document by ID |
| `docops list` | List docs with filters (`--kind`, `--status`, `--tag`) |
| `docops graph <ID>` | Typed edge graph from a starting doc |
| `docops next` | Recommend the next task to work on |
| `docops search <query>` | Substring/regex search over title, tags, body |

**Browse**

| Command | What it does |
|---|---|
| `docops html` | Emit a browsable HTML viewer to `docs/.html/` |
| `docops serve` | Localhost web viewer on `:8484` — sidebar, graph, live |

For lookups, prefer `docops list|get|search|graph|next` over loading `docs/.index.json` into agent context.

### Changing a published ADR

ADRs are append-only once accepted. Pick the right lane for the change:

| Change shape | Lane | What runs |
|---|---|---|
| Typo, dead link, errata, late-binding fact, clarification | **amend** | Add an entry under `amendments:` in the ADR's frontmatter (`kind`: `editorial` \| `errata` \| `clarification` \| `late-binding`). `docops amend` CLI verb is on the roadmap. |
| The decision itself changes (different choice, reversal, scope flip) | **supersede** | New ADR with `supersedes: [<old>]` — the back-pointer is computed by `docops index` |
| Decision stands but its scope tightens or expands | **revise** | Add a `clarification` amendment + a follow-up task if load-bearing |

The amendments **data model** ships today (validator, index, STATE, HTML viewer all understand it). See [ADR-0025](docs/decisions/ADR-0025-amendments-as-first-class-decision-metadata.md).

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