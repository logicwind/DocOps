---
title: Language-agnostic distribution — standalone binary first
status: accepted
coverage: required
date: 2026-04-22
supersedes: []
related: [ADR-0011]
tags: [distribution, packaging]
---

# Language-agnostic distribution — standalone binary first

## Context

DocOps must work in any codebase — Python, Go, Rust, Ruby, mixed, nothing-installed-yet. Requiring the user to install Bun, Node, or any ecosystem-specific runtime creates adoption friction and biases the tool to JavaScript repos.

## Decision

DocOps is distributed primarily as a **standalone binary** with no runtime dependency on the user's machine. Supported install paths:

1. **Direct binary download** from GitHub Releases (Mac/Linux/Windows).
2. **Homebrew** (`brew install docops`).
3. **Scoop/Chocolatey** (Windows).
4. **Docker image** for CI environments.
5. **Optional convenience shims** — `npx @docops/cli`, `pipx install docops`, `cargo install docops-shim` — each of which downloads and invokes the standalone binary. These are for users who prefer their language's package manager but the tool itself remains language-agnostic.

Implementation language for the CLI is an internal choice, not a user concern. The shortlist for phase 1 is:
- **Bun with `bun build --compile`** — single binary, TypeScript source, Zod ergonomics.
- **Go** — small binaries, trivial cross-compile, classic CLI choice.

Decision between these is deferred to the task that scaffolds the CLI (TP-001). The binary contract (commands, flags, output formats) is language-independent.

## Rationale

- Zero-runtime install matches the "works everywhere" promise.
- GitHub Releases binaries are trivial for CI to pull.
- Shims let language-native developers use their preferred package manager without DocOps being bound to one ecosystem.

## Consequences

- The project must invest in cross-compilation and release tooling from day one.
- No dependency on `node_modules`, `package.json`, or Python environments in the target repo.
- The binary must be small enough for fast CI pulls (< 30 MB target).
- Auto-update mechanism (`docops upgrade`) is a phase-2 consideration; phase 1 relies on the user's package manager or direct re-download.
- If a language-specific shim has a bug, it is a shim bug, not a DocOps bug — shims are thin wrappers.

## Appendix — implementation language chosen: Go

Selected during TP-001 (2026-04-22). Rationale:

- **Binary size.** `go build -ldflags "-s -w"` on a hello-world `docops --version` produced a **1.8 MB** arm64 binary — well under the 30 MB target and with massive headroom for YAML/markdown/JSON-Schema deps. A `bun build --compile` equivalent would embed the Bun runtime and start at ~55–60 MB, breaking the target before any real code is written.
- **Cross-compile.** `GOOS`/`GOARCH` gives five target platforms from a single Ubuntu runner with no toolchain gymnastics.
- **Ecosystem fit.** `gopkg.in/yaml.v3` for frontmatter, `github.com/yuin/goldmark` for markdown AST, `github.com/invopop/jsonschema` for schema emission (TP-009). All well-maintained, zero-cgo.
- **Distribution tooling.** `goreleaser` is the de-facto standard for Go CLIs and emits GitHub Releases + Homebrew formula + Scoop manifest + Docker image from one config, matching the install paths committed to above.
- **Agent authorability.** Claude / Cursor produce idiomatic Go fluently; small stdlib + explicit error handling makes review cheap.

Tradeoff accepted: Go struct tags are less ergonomic than Zod for validation. Mitigated by ADR-0002 (bare-minimum frontmatter) — the schema surface is small enough that `gopkg.in/yaml.v3` + a hand-written validator is straightforward. Richer validation machinery is not required.

### npm distribution addendum

Phase 1 ships binary + brew + scoop + direct download. npm distribution is still committed to (as an "optional convenience shim" from the decision above) and will follow the **per-platform `optionalDependencies`** pattern used by esbuild / biome / turbo: a thin `@docops/cli` meta-package plus platform packages (`@docops/cli-darwin-arm64`, `@docops/cli-linux-x64`, `@docops/cli-win32-x64`, etc.). No `postinstall` download — npm resolves the correct platform package automatically. goreleaser's `publishers` stage will emit the per-platform tarballs; a follow-up task will wire the npm publish step.
