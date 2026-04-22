---
id: "meta-index"
title: "Docs Index — How These Files Are Organized"
type: "meta"
status: "current"
last_updated: 2026-04-21
tags: [docs, conventions, index, meta]
description: "Rules and layout for the docs/ folder — the three-bucket system, ADR schema, TODO taxonomy, and lifecycle rules."
---

# Docs Index — How These Files Are Organized

This file is the map. Read it before adding, moving, or splitting anything in
`docs/`. It tells you **where things live** and **the rules that keep the folder
from drifting into chaos**.

## The three buckets

```
docs/
  reference/   ← "how it is"         stable truth, tracks code reality
  roadmap/     ← "where we're going"  forward-looking only
  archive/     ← "how it was"         shipped + historical, write-once
```

That's it. Three buckets. Anything that doesn't fit one of these doesn't belong
in `docs/`.

---

## `reference/` — how it is

Stable, current-state truth. If the code changes, update these. Out-of-date
reference docs are bugs.

```
reference/
  architecture.md      global layering (4-layer model, component map)
  daemon.md            zundd internals — data flow, state, container layout
  runtime-protocol.md  zund://stream/v1 wire format (when defined)
  decisions/           ADRs — one file per decision, dated, immutable
    0001-incus-over-docker.md
    0002-ephemeral-via-incus-copy.md
    ...
  guides/              topical how-tos — how the current system does X
    secrets.md
    cli-sequence.md
```

**Rules for reference/**

- **Code is the source of truth.** These docs summarize, they don't specify.
  If the doc says one thing and the code says another, the code wins and the
  doc gets fixed.
- **Keep each file short.** One page per concept. If it's longer, it's probably
  two concepts.
- **No roadmap language.** No "we will", "planned", "future". If you catch
  yourself writing those, it belongs in `roadmap/`, not here.
- **ADRs are immutable.** Once written, don't edit. If a decision is reversed,
  write a new ADR that references and supersedes the old one.

**ADR format** (`decisions/NNNN-short-title.md`):

ADRs carry YAML frontmatter so `scripts/adr-index.ts` can generate
`decisions/README.md` as a status / relationship / tag index. The
frontmatter is authoritative for tooling; the body is authoritative for
humans. Keep both in sync.

```markdown
---
id: "NNNN"
title: Short title
date: YYYY-MM-DD
status: proposed | accepted | superseded | rejected | draft
implementation: not-started | partial | done | n/a
supersedes: ["NNNN", ...]          # array of ADR ids (may be empty)
superseded_by: "NNNN" | null       # id of the ADR that replaced this one
related: ["NNNN", ...]             # cross-reference ADR ids
tags: [architecture, state, ...]   # faceted search keys
---

# NNNN · Short title

Date: YYYY-MM-DD
Status: accepted

## Context
What problem, what constraints.

## Decision
What we chose.

## Consequences
What this makes easier, what it makes harder.
```

**Tooling:** `bun scripts/adr-index.ts` regenerates `decisions/README.md`.
Runs automatically via lefthook pre-commit when any file in
`decisions/` changes. The script validates that all `supersedes`,
`superseded_by`, and `related` ids reference real ADRs — broken links
fail the commit.

**Status values** (decision state — orthogonal to implementation):

- `proposed` — decision drafted, not yet acted on
- `accepted` — decision is current policy
- `draft` — exploratory, not yet a real proposal
- `superseded` — no longer authoritative; see `superseded_by`
- `rejected` — considered and explicitly turned down (kept for context)

**Implementation values** (code reality — orthogonal to status):

- `not-started` — nothing in the codebase reflects the decision yet
- `partial` — some pieces landed; the decision is not fully realized
- `done` — the decision is visibly implemented in the codebase
- `n/a` — implementation state doesn't apply (e.g., superseded ADRs)

Keep `status` and `implementation` in sync with reality. An `accepted`
ADR with `implementation: not-started` is a valid state and a useful
signal — it means "we've decided, we haven't built it yet." A
`proposed` ADR with `implementation: partial` usually means the status
should flip to `accepted`.

**Adding a new ADR:**

1. Pick the next ID (`ls decisions/` to find the last one, increment).
2. Copy the frontmatter block and a previous ADR for structure.
3. Commit — lefthook regenerates the index.

**Superseding an ADR:**

1. Write the new ADR with `supersedes: ["OLD_ID"]` in frontmatter.
2. Update the old ADR's frontmatter: `status: superseded`,
   `superseded_by: "NEW_ID"`.
3. Do not edit the body of the old ADR — the archive rule applies to
   superseded ADRs too.

---

## `roadmap/` — where we're going

Forward-looking only. Nothing that has shipped lives here.

```
roadmap/
  vision.md        north star — what Zund becomes long-term
  current.md       active slice/milestone (only one at a time)
  next.md          near-term priorities — bullet-level, not specs
```

**Rules for roadmap/**

- **Vision is aspirational.** It can outrun reality. But it should be coherent —
  a reader should finish it knowing what Zund is trying to become.
- **Only one `current.md`.** The active slice. When it ships, move it to
  `archive/slice-N-plan.md` and write the next one.
- **`next.md` is a bulleted parking lot**, not a spec. 2–3 near-term
  priorities, each a sentence or two. If an item grows into a full plan, it
  becomes the next `current.md`.
- **No TODOs.** Task-level work doesn't live here. See "Where TODOs go" below.

---

## `archive/` — how it was

Write-once, read-sometimes. Preserves context about shipped work.

```
archive/
  slice-5-plan.md
  slice-6-plan.md
  slice-7-plan.md
  zund-plan-v0.3.md     superseded planning docs, kept for context
  ...
```

**Rules for archive/**

- **Don't delete.** Preserves the "why we did it this way" context that the
  code can't show on its own.
- **Don't maintain.** Once here, a file is frozen. Don't update it to match
  later changes. If something in archive is still relevant, extract it into
  a new ADR or reference doc.
- **Archive eagerly.** When a slice ships, move its plan here the same day.
  The signal that we're done is the move.

---

## Where TODOs go (not here)

Task-level work does **not** belong in markdown. Pick the right tool:

| Scope | Where it lives |
|-------|----------------|
| Line-level ("fix this here") | `// TODO:` comment in code |
| Feature-level ("build X") | GitHub issue |
| Slice-level ("next chunk of work") | `roadmap/current.md` |
| Cross-cutting idea ("some day") | `roadmap/next.md` |

The reason: **markdown TODO lists rot silently.** Git log and a shipped
archive tell you what got done. Don't maintain parallel checklists.

---

## Lifecycle rules

**When a slice ships:**
1. Move `roadmap/current.md` → `archive/slice-N-plan.md`
2. Extract any irreversible architectural choices into
   `reference/decisions/NNNN-*.md` as ADRs
3. Write the next slice plan at `roadmap/current.md`

**When a decision is made in conversation:**
- Small tactical choice → goes in the slice plan
- Architectural choice with long-term implications → write an ADR in
  `reference/decisions/` the same session. Decisions that only live in chat
  logs are decisions that will be forgotten.

**When reference docs feel stale:**
- Fix them the same PR that changed the code. Don't accumulate drift.

---

## What stays at the repo root (not in `docs/`)

- `CLAUDE.md` — AI code conventions
- `HOW-TO-CLAUDE-CODE.md` — AI workflow playbook
- Package-level `README.md` — per-package orientation

These are AI-facing or per-package and aren't "documentation" in the same
sense. They live where the tools find them.

---

## What stays in `docs/` at root level

- `index.md` — this file
- `braindump.md` — unstructured notes, pre-bucket scratchpad (see below)
- `REFERENCES.yaml` — external links referenced elsewhere

Nothing else should accumulate at the root of `docs/`. If it's a doc, it
belongs in one of the three buckets or the braindump.

---

## `braindump.md` — pre-structured thinking

Not a bucket. A scratchpad for ideas that aren't ready to commit to a
shape yet. Sits outside the reference / roadmap / archive system because
it's explicitly *un*structured.

**Rules:**

- **Date each entry.** `## YYYY-MM-DD — short title`, newest at the top.
- **Any format goes.** Fragments, bullets, one-liners. Don't polish.
- **Graduation removes the entry.** When a thought becomes an ADR, a
  roadmap item, or is rejected, delete it from braindump. The file is a
  working edge, not a history.
- **Not for anything shipped, active, or decided.** Those have their own
  buckets.

---

## Quick decision tree

```
Is it about how the system works TODAY?          →  reference/
  ...and it's a locked architectural choice?     →  reference/decisions/
  ...and it's a how-to for one topic?            →  reference/guides/
Is it about what we're GOING TO BUILD?           →  roadmap/
  ...and it's the active slice?                  →  roadmap/current.md
  ...and it's near-term parking lot?             →  roadmap/next.md
  ...and it's long-term vision?                  →  roadmap/vision.md
Is it about what was SHIPPED?                    →  archive/
Is it a task-level TODO?                         →  not in docs/ (see above)
Is it an unformed idea you don't want to lose?   →  braindump.md
```
