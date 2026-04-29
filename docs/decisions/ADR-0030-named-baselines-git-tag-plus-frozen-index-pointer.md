---
title: Named baselines — git tag plus frozen index pointer
status: draft
coverage: required
date: "2026-04-30"
supersedes: []
related: [ADR-0018, ADR-0025, ADR-0029]
tags: []
---

## Context

Users have asked for a way to "lock version 1" of a project's decision graph — a stable cut they can refer back to as truth at a point in time, especially when contrasting it against later changes. The naive approach is to *copy* every CTX, ADR, and TP into a `v1/` folder and freeze them. That works but is heavyweight, fragments truth (which file is current?), inflates the index, and makes queries like "what's the decision today?" harder, not easier.

DocOps already has the substrate for a much lighter answer:

1. Git already snapshots every doc at every commit; that's the storage layer.
2. The computed index (`docs/.index.json`) is the queryable shape of the graph at a moment in time.
3. ADR-0025 (amendments) and the natural supersession chain already preserve decision history *inside* the live graph — so users don't need to copy docs to "see what we decided then."

What's missing is a lightweight **named cut**: a way to say "as of release v1, this set of doc IDs at these git SHAs were the agreed truth," without duplicating any content.

## Decision

A baseline is a **name + git tag + frozen index pointer**, written by `docops baseline create`:

1. Tags the current commit `release/<name>` (e.g. `release/v1`).
2. Writes `docs/baselines/<name>.json` — a frozen snapshot of `docs/.index.json` at that moment, plus the resolved git SHA per included doc.
3. Optionally accepts `--message` (stored in the baseline file) for the human-readable changelog narrative; or pairs with an authored `CTX-release-<name>` if the user wants a fuller release note.

CLI surface (skills, not slash commands per ADR-0029):

```
docops baseline create <name> [--message ...] [--include kind=ADR,CTX,TP]
docops baseline list
docops baseline show <name>
docops baseline diff <a> <b>          # what moved between two baselines
docops baseline current               # which baseline (if any) is the working set "after"
```

Resolution: `docops get <ID> --at <baseline>` prints the doc as of that baseline's git SHA (via `git show`), so the LLM and humans can both ask "what did ADR-0009 say at release/v1?" without leaving the CLI.

Baselines are append-only — you can delete a tag, but the contract is that a published baseline is immutable. Re-cutting a release uses a new name (`v1.1`) rather than overwriting `v1`.

## Rationale

- **Truth stays in one graph.** The live tree under `docs/` is always "current"; baselines are pointers, not copies. No "which file is canonical" confusion.
- **Cheap to create, cheap to compare.** A baseline is one JSON file plus a git tag. `diff` is set algebra over IDs + SHA comparison — fast, scriptable, useful in release notes.
- **Composes with amendments (ADR-0025) and supersession.** Reading an ADR "at v1" gives you what it said *then*, including amendments accepted by then. The history model and the snapshot model don't fight.
- **Composes with command-surface tiering (ADR-0029).** No new slash command needed; baselines live as a skill (`docops:baseline`) and CLI verb. User-facing "moments" reach baselines through `/docops:do` ("cut release v1") or by typing `docops baseline create v1` directly.
- **Git-native.** Anyone reviewing the repo with normal Git tools can already see the tag, fetch the snapshot file, and reproduce the state. No DocOps-only artifacts.

## Consequences

- New CLI verb group `baseline` (5 subcommands: create, list, show, diff, current). New skill `docops:baseline`. No new slash command.
- New folder `docs/baselines/` (gitignore-safe? — no, baselines must be committed to be useful as shared truth). Validator should ignore the folder for graph rules but ensure baseline files conform to a small JSON schema.
- `docops get --at <baseline>` requires `git show` access; works in any cloned repo, fails gracefully outside a git checkout.
- Tag namespace `release/*` is reserved; users with existing tags in that namespace need a migration note.
- A future `docops baseline lock` could enforce "no edits to docs included in the most recent baseline without a follow-up ADR" — out of scope here, but the structure supports it.
- Pairs with a forthcoming task to implement: TP for `baseline create|list|show|diff|current`; TP to extend `docops get` with `--at`; TP for the JSON schema for baseline files.
