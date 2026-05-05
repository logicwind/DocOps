---
title: Next-step affordances on every mutating CLI command
status: done
priority: p2
assignee: claude
requires: [ADR-0033]
depends_on: []
---

## Goal

Every command that mutates state ends with a `→ Next:` block listing
1–3 concrete follow-up commands. Suppressed under `--json` and
`--quiet`.

## Scope

1. **New package** `internal/nextsteps`:
   - `Suggest(verb string, ctx Outcome) []Step` returns labelled
     copy-pasteable commands.
   - `Step{Label, Command}` and a `Render(io.Writer, []Step)` helper
     that prints the standard block.
   - Per-verb tables live in the package; new verbs register their
     suggestions there.
2. **Wire into every mutating command** (initial set):
   - `init` — driven by brownfield/greenfield detection (other TP).
   - `new ctx`, `new adr`, `new task` — point at the next natural
     step (e.g., new task after new adr, refresh after new task).
   - `refresh`, `amend`, `supersede`, `revise` — point at validation
     and downstream review.
   - `audit`, `state` — point at the most-likely next mutation
     (e.g., `docops new task --requires <gap>`).
3. **Global `--quiet` flag** on the root cobra command. When set,
   suppresses next-step blocks across all verbs.
4. **`--json` output**: every command that already supports `--json`
   gains a `next_steps: [{label, command}]` field, and the human
   block is suppressed automatically.
5. **Style guide** — short comment in `internal/nextsteps/doc.go`
   describing the bar for a good suggestion: must be the *most likely*
   next move, never more than 3, never "see docs" punts.

## Out of scope

- Auto-derivation of suggestions from the doc graph. Each verb
  declares its own — keeps them high-signal.
- Localization or theming.
- Suggestions on read commands (`get`, `list`, `search`, `graph`,
  `next`). Reads are noiseless by design (ADR-0018).

## Done when

- All listed mutating verbs print a `→ Next:` block by default.
- `--quiet` suppresses; `--json` swaps the block for
  `next_steps[]`.
- One unit test per verb confirms the suggestion list stays in sync.
- README "Getting started" section gets a one-paragraph rewrite
  pointing brownfield users at `/docops:onboard` and greenfield users
  at `/docops:plan`. (Tracked here because it's the natural moment to
  do it; can be split if too large.)
