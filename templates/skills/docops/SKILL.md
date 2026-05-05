---
name: docops
description: Use when working with a DocOps-managed repository — querying or mutating decisions, context docs, and tasks. Triggers on doc IDs (ADR-NNNN, CTX-NNN, TP-NNN), the words amend/supersede/revise/audit/refresh, or requests to list, search, look up, plan, or extend project state. Reach for the docops CLI before reading docs/.index.json or grepping the doc tree.
---

# DocOps skill

Umbrella router. Detailed procedures live in [cookbook/](cookbook/) — read
the chapter that matches the user's intent. Prefer the `docops` CLI over
loading `docs/.index.json` directly.

## Doc taxonomy

| Kind | Lives in | Holds |
|---|---|---|
| `CTX-NNN` | `docs/context/` | PRDs, design, memos, research, guardrails |
| `ADR-NNNN` | `docs/decisions/` | Architecture decisions (frontmatter is load-bearing) |
| `TP-NNN` | `docs/tasks/` | Work units; each cites ≥1 ADR or CTX in `requires:` |

Filename is the ID. There is no `id:` field.

## Invariants

1. Every task cites ≥1 ADR or CTX in `requires:`. Validator enforces.
2. References must resolve. No non-existent or dangling-superseded refs.
3. Don't edit `docs/STATE.md` or `docs/.index.json` — both are regenerated.
4. Don't edit reverse-edge fields in frontmatter — computed in the index.

## Cookbook — read the chapter that matches the user's intent

| User intent | Read |
|---|---|
| "what's the state?" | [state](cookbook/state.md), [audit](cookbook/audit.md) |
| References a specific ID | [get](cookbook/get.md), [graph](cookbook/graph.md) |
| Find or browse | [search](cookbook/search.md), [list](cookbook/list.md) |
| Start work | [next](cookbook/next.md), [do](cookbook/do.md), [plan](cookbook/plan.md) |
| Finish a chunk of work | [close](cookbook/close.md), [progress](cookbook/progress.md), [refresh](cookbook/refresh.md) |
| Add new docs | [new-adr](cookbook/new-adr.md), [new-ctx](cookbook/new-ctx.md), [new-task](cookbook/new-task.md) |
| Typo / dead link / late-binding fact in a published ADR | [amend](cookbook/amend.md) |
| Replace a published ADR with a new decision | [supersede](cookbook/supersede.md) |
| Tighten / expand an ADR's scope without flipping the call | [revise](cookbook/revise.md) |
| Bootstrap or upgrade docops | [init](cookbook/init.md), [upgrade](cookbook/upgrade.md) |
| Bootstrap CTX + ADRs from an existing codebase | [onboard](cookbook/onboard.md) |

## Changing a published ADR — pick the lane first

| Change shape | Lane |
|---|---|
| Typo, dead link, rename, late-binding fact, errata, clarification | **amend** |
| Decision changes (different choice, reversal, scope flip) | **supersede** |
| Decision stands; scope tightens or expands | **revise** |

If unsure, ask one clarifying question before acting.

## Workflow

1. `docops state` (or read `docs/STATE.md`).
2. `docops audit` for open gaps.
3. Pick a task; check `depends_on`.
4. Before coding: read every doc in the task's `requires:` and `depends_on:`.
5. Work in your native plan/execute mode.
6. After: update `task_status:` and run `docops refresh`.
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
docops amend <ADR-ID> --kind <kind> --summary "..." [--ref ...] [--by NAME]
```

For lookups, prefer `docops list|get|search|graph|next` over loading
`docs/.index.json` into context.
