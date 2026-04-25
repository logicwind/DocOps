---
name: docops
description: Use when working with a DocOps-managed repository — querying or mutating decisions, context docs, and tasks. Triggers on doc IDs (ADR-NNNN, CTX-NNN, TP-NNN) or requests to list, search, look up, refresh, or extend project state. Reach for the docops CLI before reading docs/.index.json or grepping the doc tree.
---

# DocOps skill

This repo uses **DocOps** — a typed project-state substrate built on
markdown files with structured YAML frontmatter. The `docops` CLI is
the canonical query and mutation interface; prefer it over loading
`docs/.index.json` directly.

## Doc taxonomy

| Kind | Lives in | Holds |
|---|---|---|
| `CTX-NNN` | `docs/context/` | PRDs, design notes, memos, research, guardrails |
| `ADR-NNNN` | `docs/decisions/` | Architecture decisions (frontmatter is load-bearing) |
| `TP-NNN` | `docs/tasks/` | Work units; each cites ≥1 ADR or CTX in `requires:` |

Filename is the ID. `ADR-0019-foo.md` is `ADR-0019`. There is no `id:` field.

## Invariants

1. Every task cites ≥1 ADR or CTX in `requires:`. Validator enforces.
2. References must resolve. No non-existent or dangling-superseded refs.
3. Don't edit `docs/STATE.md` or `docs/.index.json` — both are regenerated.
4. Don't edit reverse-edge fields in frontmatter — computed in the index.

## When to load which subroutine

The bundle root (this file) is the orientation entry point. Read the
specific subroutine when the user's request matches its scope.

| User intent | Read |
|---|---|
| "what's the state of the project?" | [state.md](state.md), [audit.md](audit.md) |
| References a specific ID | [get.md](get.md), [graph.md](graph.md) |
| Wants to find or browse | [search.md](search.md), [list.md](list.md) |
| Wants to start work | [next.md](next.md), [do.md](do.md), [plan.md](plan.md) |
| Finishes a chunk of work | [close.md](close.md), [progress.md](progress.md), [refresh.md](refresh.md) |
| Adds new docs | [new-adr.md](new-adr.md), [new-ctx.md](new-ctx.md), [new-task.md](new-task.md) |
| Bootstraps or upgrades docops | [init.md](init.md), [upgrade.md](upgrade.md) |

## Workflow

1. `docops state` (or read `docs/STATE.md`).
2. `docops audit` for open gaps.
3. Pick a task from `docs/tasks/`; check `depends_on`.
4. Before coding: read every doc in the task's `requires:` and `depends_on:`.
5. Work in your native plan/execute mode.
6. After: update the task's `status:` and run `docops refresh`.
7. New decision → `docops new adr`. New gap → `docops new task` with citations.

## CLI cheatsheet

All commands support `--json`.

```
docops state | audit | refresh | validate | index
docops list [--kind CTX|ADR|TP] [--status ...] [--tag ...]
docops get <ID>
docops graph <ID>
docops next
docops search <query>
docops new ctx|adr|task "title" [--body -|TEXT | --body-file PATH]
```

For lookups, prefer `docops list|get|search|graph|next` over loading
`docs/.index.json` into context.
