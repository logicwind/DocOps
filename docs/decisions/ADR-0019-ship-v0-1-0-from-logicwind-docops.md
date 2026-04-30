---
title: Ship v0.1.0 from logicwind/DocOps
status: accepted
coverage: required
date: 2026-04-22
supersedes: []
related: [ADR-0011, ADR-0012, ADR-0013, ADR-0014]
tags: [release, distribution, org, scope]
amendments:
  - date: 2026-04-23
    kind: editorial
    by: nix
    summary: "Tap/bucket repo names: per-tool → org-wide convention"
    affects_sections: ["v0.1.0 scope"]
    ref: TP-024
---

# Ship v0.1.0 from logicwind/DocOps

## Context

The phase-1 CLI surface (`init`, `validate`, `index`, `state`, `audit`,
`new`, `schema`) has landed across TP-001…TP-009. The project has so far
been developed under the placeholder module path `github.com/nachiket/docops`
because no decision had been made about the publishing org. The README,
`.goreleaser.yml`, and skill scaffolding all still point at this stub
owner. Before a public release can happen, three things need to be
nailed down:

1. The canonical GitHub org and module path.
2. The license.
3. What counts as in-scope for v0.1.0 — specifically, whether the
   "standalone skill pack" concept from ADR-0013 is a v0.1.0 deliverable
   or can be deferred without regret.

## Decision

### Org and module path

DocOps ships under `github.com/logicwind/DocOps`. The **repository name**
is `DocOps` (CamelCase, matches the product name and the `docops` CLI).
The **Go module path** is `github.com/logicwind/docops` (lowercase).
GitHub resolves both cases to the same repository, so cloning,
go-get, and the module proxy all work. Lowercase in the module path
matches Go ecosystem convention and avoids awkward mixed-case imports.

Every Go source file, `go.mod`, README install snippet,
`.goreleaser.yml` owner field, and skill-template reference migrates
from `nachiket/docops` to `logicwind/docops` (or `logicwind/DocOps`
for user-facing URLs) in a single commit. No compatibility shim for
the old path — there is no installed user base to preserve.

### License

MIT. A `LICENSE` file lands at the repo root with the standard MIT
text, copyright holder `Logicwind Technologies Pvt Ltd` (and/or the
org's legal entity name). The README's "pending" line is removed.

### v0.1.0 scope

In scope:

- `docops init / validate / index / state / audit / new / schema`.
- Auto-scaffolded `/docops:*` agent skills for Claude Code and Cursor
  (via init; see the skill-pack descope below).
- JSON Schema output for editors (TP-009).
- goreleaser pipeline producing binaries + archives + checksums for
  darwin/linux/windows on amd64/arm64, plus a GHCR image.

Explicitly **out of scope** for v0.1.0:

- `docops next / get / list / graph / status / search / review` — these
  ship incrementally in later point releases. README and AGENTS.md
  already mark them as "coming".
- Homebrew tap (`logicwind/homebrew-docops`) and Scoop bucket
  (`logicwind/scoop-docops`) repositories — the goreleaser job
  currently references them but publish is gated on those repos
  existing. v0.1.0 ships without auto-published tap/bucket; users
  install via direct GitHub Release download until the tap lands in
  a follow-up. `[AMENDED 2026-04-23 editorial]` — these repos were
  migrated to the org-wide `logicwind/homebrew-tap` and
  `logicwind/scoop-bucket` pre-launch; see the Amendments section
  below.
- npm per-platform shim packages (deferred per ADR-0012 addendum).

### Skill-pack descope (amends ADR-0013)

ADR-0013 described two distribution paths for agent skills:

1. Standalone skill pack (`@docops/skills-claude-code`,
   `@docops/skills-cursor`) as installable npm packages.
2. Auto-scaffolded into user repos on `docops init`.

Path 2 (init-based) has shipped in TP-007 and is the only path users
actually need. The standalone-pack layer (`packages/skills-claude-code/`,
`packages/skills-cursor/` in the source tree) is deferred to post-v0.1.0
and **not** a v0.1.0 blocker. Rationale: no user has asked for skills
without init; maintaining a second copy at `packages/` risks drift from
`templates/skills/docops/`; npm distribution was already descoped for
the CLI itself under ADR-0012.

TP-010's acceptance is rescoped accordingly in TP-013: drop the
`packages/` layout, drop skills for unshipped CLI commands
(`docops-review`, `docops-graph`), keep `--no-skills` and a basic CI
lint, and use `templates/skills/docops/` as the single source of truth.

### Release mechanics

- Tag `v0.1.0` on `main` triggers `.github/workflows/release.yml` which
  runs goreleaser. No manual artifact builds for the first release.
- `.goreleaser.yml` already emits the matrix; only owner fields need
  updating for the new org.
- The tap and bucket publishers in goreleaser are `disable: true` for
  v0.1.0 (flipped on when those repos exist).

## Rationale

- Shipping under the org the project will actually live in avoids a
  second path migration later and lets us publish tap/bucket/image
  under the correct namespace from day one.
- MIT is the least-friction license for a developer tool that expects
  to be dropped into any repo. Apache-2.0 would buy us explicit patent
  grants but the surface here is too small to justify the extra header
  ceremony.
- Descoping the `packages/` skill layout avoids solving a
  not-yet-requested problem. If demand appears, we can revive it as a
  follow-up ADR; deleting duplicate content later is cheap.
- Holding the tap/bucket to a follow-up release keeps v0.1.0 free of
  cross-repo dependencies that could break the first tag.

## Consequences

- Module path migration is mechanical but touches every Go file —
  TP-014 handles it atomically.
- README install snippets change; the "publishes once v0.1.0 ships"
  hedging disappears for the GitHub Releases path and remains for
  tap/bucket.
- Any future contributor cloning under the wrong path sees immediate
  build errors; there is no silent compat shim.
- `.goreleaser.yml` brew/scoop publishers stay disabled until tap and
  bucket repos exist — that is a deliberate phase-2 decision recorded
  here so future agents don't try to "fix" the disabled flag.
- The skill-pack descope leaves TP-010 partially subsumed by TP-007;
  TP-013 closes the remaining gaps (`--no-skills`, CI lint) so TP-010
  can be marked done cleanly.

## Amendments

Editorial corrections after this ADR was accepted. The decision
substance is unchanged; factual details (names, links, references)
are refreshed. Once ADR-0025 / TP-026 ship the `amendments:`
frontmatter schema, this section becomes machine-readable (see
TP-027 for the backfill).

### 2026-04-23 — Tap/bucket repo naming (editorial)

**Summary.** The v0.1.0 "out of scope" section named the follow-up
tap and bucket repos `logicwind/homebrew-docops` and
`logicwind/scoop-docops`. Those repos were indeed created on
2026-04-22 with hand-authored formulas for v0.1.0 and v0.1.1. On
2026-04-23 — still pre-launch, with no external users pinned to
the install path — we migrated to the org-wide convention
(`logicwind/homebrew-tap` and `logicwind/scoop-bucket`), matching
the Vercel / HashiCorp / Fly.io pattern. TP-024 tracks the
mechanical migration (create new repos, seed with v0.1.1 formula,
archive the legacy per-tool repos, flip goreleaser's `skip_upload`).

**What the decision said.** Defer tap/bucket setup to a follow-up
release.

**What is unchanged.** The deferral posture. The phase-2 gate. The
goreleaser `skip_upload: true` until TP-024 lands.

**What changed.** Only the target repo names, and the rationale
captured here. No re-vote required.

**By.** nix (unilateral editorial correction — pre-launch timing
means the install-path change has zero blast radius beyond the
author's own machine).
