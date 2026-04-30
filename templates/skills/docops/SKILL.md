---
name: docops
description: Use when working with a DocOps-managed repository — querying or mutating decisions, context docs, and tasks. Triggers on doc IDs (ADR-NNNN, CTX-NNN, TP-NNN), the words amend/supersede/revise/audit/refresh, or requests to list, search, look up, plan, or extend project state. Reach for the docops CLI before reading docs/.index.json or grepping the doc tree.
---

# DocOps skill

This repo uses **DocOps** — a typed project-state substrate built on
markdown files with structured YAML frontmatter. The `docops` CLI is
the canonical query and mutation interface; prefer it over loading
`docs/.index.json` directly.

This file is the umbrella router. Detailed procedures live in
[cookbook/](cookbook/) — read the chapter that matches the user's intent.

## Variables

> Defaults match the standard DocOps layout. If `docops.yaml` overrides
> `docs_dir:`, substitute that path everywhere these tokens appear.

- **DOCOPS_DOCS_DIR**: `docs/`
- **DOCOPS_CONTEXT_DIR**: `docs/context/`
- **DOCOPS_DECISIONS_DIR**: `docs/decisions/`
- **DOCOPS_TASKS_DIR**: `docs/tasks/`
- **DOCOPS_BIN**: `docops`

## Doc taxonomy

| Kind | Lives in | Holds |
|---|---|---|
| `CTX-NNN` | `<DOCOPS_CONTEXT_DIR>` | PRDs, design notes, memos, research, guardrails |
| `ADR-NNNN` | `<DOCOPS_DECISIONS_DIR>` | Architecture decisions (frontmatter is load-bearing) |
| `TP-NNN` | `<DOCOPS_TASKS_DIR>` | Work units; each cites ≥1 ADR or CTX in `requires:` |

Filename is the ID. `ADR-0019-foo.md` is `ADR-0019`. There is no `id:` field.

## Invariants

1. Every task cites ≥1 ADR or CTX in `requires:`. Validator enforces.
2. References must resolve. No non-existent or dangling-superseded refs.
3. Don't edit `docs/STATE.md` or `docs/.index.json` — both are regenerated.
4. Don't edit reverse-edge fields in frontmatter — computed in the index.

## Cookbook — when to read which chapter

The umbrella (this file) is the orientation entry point. Open the
cookbook chapter when the user's request matches its scope.

| User intent | Read |
|---|---|
| "what's the state of the project?" | [cookbook/state.md](cookbook/state.md), [cookbook/audit.md](cookbook/audit.md) |
| References a specific ID | [cookbook/get.md](cookbook/get.md), [cookbook/graph.md](cookbook/graph.md) |
| Wants to find or browse | [cookbook/search.md](cookbook/search.md), [cookbook/list.md](cookbook/list.md) |
| Wants to start work | [cookbook/next.md](cookbook/next.md), [cookbook/do.md](cookbook/do.md), [cookbook/plan.md](cookbook/plan.md) |
| Finishes a chunk of work | [cookbook/close.md](cookbook/close.md), [cookbook/progress.md](cookbook/progress.md), [cookbook/refresh.md](cookbook/refresh.md) |
| Adds new docs | [cookbook/new-adr.md](cookbook/new-adr.md), [cookbook/new-ctx.md](cookbook/new-ctx.md), [cookbook/new-task.md](cookbook/new-task.md) |
| Fix typo / dead link / late-binding fact in a published ADR | [cookbook/amend.md](cookbook/amend.md) |
| Replace a published ADR with a new decision | [cookbook/supersede.md](cookbook/supersede.md) |
| Tighten or expand the scope of a published ADR without flipping the call | [cookbook/revise.md](cookbook/revise.md) |
| Bootstraps or upgrades docops | [cookbook/init.md](cookbook/init.md), [cookbook/upgrade.md](cookbook/upgrade.md) |

## Amendment-vs-Supersede-vs-Revise — pick the right lane

When a published ADR needs to change, choose **before** reaching for any
chapter. The three lanes are not interchangeable.

| Change shape | Lane | Why |
|---|---|---|
| Typo, dead link, rename pass-through, late-binding fact, errata, clarification | **amend** | Decision unchanged. Audit trail kept on the ADR. No new ADR. |
| The decision itself changes (different choice, reversal, scope flip) | **supersede** | Old ADR remains historical; a new ADR records the new decision. |
| Decision stands but its scope tightens or expands without flipping the call | **revise** | Lighter than supersede; documents scope drift without churning ADR numbers. |

If unsure, ask the user one clarifying question before acting (per
ADR-0029's dispatcher rubric).

## Workflow

1. `<DOCOPS_BIN> state` (or read `<DOCOPS_DOCS_DIR>STATE.md`).
2. `<DOCOPS_BIN> audit` for open gaps.
3. Pick a task from `<DOCOPS_TASKS_DIR>`; check `depends_on`.
4. Before coding: read every doc in the task's `requires:` and `depends_on:`.
5. Work in your native plan/execute mode.
6. After: update the task's `status:` and run `<DOCOPS_BIN> refresh`.
7. New decision → `<DOCOPS_BIN> new adr`. New gap → `<DOCOPS_BIN> new task` with citations.

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
docops amend <ADR-ID> --kind <kind> --summary "..." [--ref TP-NNN] [--by NAME]
```

For lookups, prefer `docops list|get|search|graph|next` over loading
`docs/.index.json` into context.
