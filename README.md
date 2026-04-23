# docops

Typed project-state substrate for LLM-first software development. Three doc types (Context, Decision, Task) in markdown + YAML frontmatter, a computed index, a small CLI, and a coverage audit — designed so any coding agent can load a repo and know what's been decided, what's pending, and what to do next.

**Status:** v0.1.0 — `init`, `validate`, `index`, `state`, `audit`, `new`, `schema` are shipped. `next`, `get`, `list`, `graph`, `status`, `search`, `review` are on the roadmap. See `docs/STATE.md` for the current backlog.

## Install

### macOS / Linux (Homebrew)

```sh
brew install logicwind/docops/docops
```

### Windows (Scoop)

```sh
scoop bucket add docops https://github.com/logicwind/scoop-docops
scoop install docops
```

### Direct download

Grab the archive for your platform from [GitHub Releases](https://github.com/logicwind/DocOps/releases), extract, put `docops` on your PATH.

### Docker (planned)

A GHCR image lands in a follow-up release. Until then, use Homebrew, Scoop, or direct download.

### npm shim (planned)

Per-platform packages (`@docops/cli-darwin-arm64`, `@docops/cli-linux-x64`, ...) will publish alongside a future release. `npm i -g @docops/cli` resolves the matching native binary via `optionalDependencies` — no postinstall network fetch. See ADR-0012 for distribution rationale.

### Upgrading an existing project

After `brew upgrade docops` (or your package manager equivalent), pull the
new binary's shipped templates into your project without clobbering
`docops.yaml` or your pre-commit hook:

```sh
brew upgrade docops          # or scoop update docops, etc.
docops upgrade               # syncs skills, schemas, AGENTS.md block
docops upgrade --dry-run     # preview first if you prefer
```

`docops upgrade` only touches DocOps-owned scaffolding. To also rewrite
`docops.yaml` or reinstall the pre-commit hook, opt in with `--config`
or `--hook`. Run `docops update-check` (or wait for `docops upgrade` to
warn you on its own) to learn when a new release is available.

## Smoke test

```sh
docops --version
docops --help
```

## Quickstart — use DocOps in your own repo

From the root of any git repo (empty or existing):

```sh
docops init                                        # scaffolds docs/, docops.yaml, schemas, skills, pre-commit hook, AGENTS.md + CLAUDE.md
docops new ctx "Vision" --type brief --no-open     # first CTX
docops new adr "Pick a database"                   # first decision
docops new task "Wire up SQLite" --requires ADR-0001
docops validate                                    # schema + graph invariants
docops index                                       # writes docs/.index.json
docops state                                       # writes docs/STATE.md
docops audit                                       # structural gap report
```

`docops init --dry-run` previews; `docops init --force` re-syncs drifted scaffolded files; `docops init --no-skills` skips the agent-skill scaffolding.

## Editor integration

`docops init` (and `docops schema`) write three JSON Schema files under `docs/.docops/schema/`:

- `context.schema.json` — CTX frontmatter (includes a `type:` enum driven by `context_types:` in `docops.yaml`)
- `decision.schema.json` — ADR frontmatter
- `task.schema.json` — Task frontmatter

Install the [`redhat.vscode-yaml`](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml) extension, then add to your workspace `.vscode/settings.json`:

```json
"yaml.schemas": {
  "./docs/.docops/schema/context.schema.json":  "docs/context/*.md",
  "./docs/.docops/schema/decision.schema.json": "docs/decisions/*.md",
  "./docs/.docops/schema/task.schema.json":     "docs/tasks/*.md"
}
```

After editing `context_types:` in `docops.yaml`, run `docops schema` to regenerate the schemas without re-running a full `docops init`.

## Developing on DocOps itself

This repository is the DocOps **source**, and it dog-foods its own convention for its own project management. Before changing anything, read `AGENTS.md` in the root — it separates the "meta" side (this repo's own docs) from the "product" side (what we ship to users). See `docs/decisions/ADR-0016-meta-vs-product-separation.md`.

### Local workflow

```sh
make tidy     # go mod tidy
make build    # builds bin/docops
make test     # go test -race ./...
make lint     # go vet ./...
```

### Release

Tag a commit with `vX.Y.Z`; the `Release` workflow runs goreleaser, which builds the matrix, attaches archives + checksums to the GitHub Release, and updates the brew/scoop stubs (once those repos exist).

```sh
git tag v0.1.0
git push origin v0.1.0
```

For a dry run:

```sh
make release-snapshot
```

## License

MIT — see LICENSE.
