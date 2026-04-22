# docops

Typed project-state substrate for LLM-first software development. Three doc types (Context, Decision, Task) in markdown + YAML frontmatter, a computed index, a small CLI, and a coverage audit — designed so any coding agent can load a repo and know what's been decided, what's pending, and what to do next.

**Status:** phase 1. Only the CLI scaffold is in place (TP-001). Schemas, commands, and docs land in TP-002 onward. See `docs/STATE.md` for the current backlog.

## Install

### macOS / Linux (Homebrew)

```sh
brew install nachiket/docops/docops
```

> Tap publishes once `v0.1.0` ships.

### Windows (Scoop)

```sh
scoop bucket add docops https://github.com/nachiket/scoop-docops
scoop install docops
```

> Bucket publishes once `v0.1.0` ships.

### Direct download

Grab the archive for your platform from [GitHub Releases](https://github.com/nachiket/docops/releases), extract, put `docops` on your PATH.

### Docker

```sh
docker run --rm -v "$PWD:/repo" ghcr.io/nachiket/docops:latest --version
```

> Image publishes once `v0.1.0` ships.

### npm shim (planned)

Per-platform packages (`@docops/cli-darwin-arm64`, `@docops/cli-linux-x64`, ...) will publish alongside each release. `npm i -g @docops/cli` resolves the matching native binary via `optionalDependencies` — no postinstall network fetch. See ADR-0012 for distribution rationale.

## Smoke test

```sh
docops --version
```

Everything else is pending — read `docs/tasks/` for the phase-1 backlog.

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

MIT (pending — a LICENSE file will land with `v0.1.0`).
