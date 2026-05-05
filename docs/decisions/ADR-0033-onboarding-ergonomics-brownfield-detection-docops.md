---
title: Onboarding ergonomics — brownfield detection, /docops:onboard, next-step affordances
status: accepted
coverage: required
date: "2026-05-05"
supersedes: []
related: [ADR-0020, ADR-0029, ADR-0031]
tags: []
---

## Context

`docops init` is currently a quiet, additive scaffold: it writes
`docs/{context,decisions,tasks}/`, `docops.yaml`, schemas, and the
CLAUDE.md / AGENTS.md blocks, then exits. Nothing tells the user what
to do next, and nothing notices whether the repo is empty (greenfield)
or already has years of code (brownfield).

That gap shows up in two ways:

1. **Brownfield friction.** Drop DocOps into a 5-year-old codebase and
   you face a blank `docs/` folder with no obvious path back to the
   load-bearing decisions already encoded in the code. Adoption stalls
   on "where do I even start?".
2. **Greenfield friction.** Even on a fresh repo, `init` ends in
   silence — the user has to leaf through the README to find the next
   command. The blank-page problem hits agents and humans equally.

The substrate is fine; the **on-ramp** is missing. This ADR designs
the on-ramp.

## Decision

Three coordinated additions:

### 1. Brownfield detection in `docops init`

After scaffolding, `init` runs a cheap heuristic scan of the working
directory and routes its closing message accordingly:

- **Brownfield** if any of: `package.json`, `go.mod`, `Cargo.toml`,
  `pyproject.toml`, `Gemfile`, `pom.xml`, `composer.json`,
  `requirements.txt`, top-level `src/` or `app/` or `lib/`, *or* >10
  git commits at HEAD.
- **Greenfield** otherwise.

Detection is pure file/git inspection — no AI call, no network — and
adds <50ms to `init`. Output:

```
DocOps initialized in ./docs.

Existing code detected (go.mod, src/, 184 commits).
→ Next:
  • /docops:onboard      bootstrap CTX + ADRs from this codebase
  • docops new ctx --type brief "..."   start fresh from a blank brief
```

Greenfield prints the same block with `/docops:plan` and
`docops new ctx` reversed.

### 2. `/docops:onboard` skill

A new skill (ADR-0029 "moment" tier — high-level user intent, lives
in the umbrella `docops` skill bundle per ADR-0031). **Agent-driven, no
new CLI verb.** The skill instructs the agent to:

1. Inspect top-level layout: README, package manifests, `git log -20`,
   primary directories. Build an inferred summary.
2. Show the summary back: "Looks like a Next.js + Clerk app, ~400
   commits, primary surface seems to be `<X>`."
3. Run a goal-oriented interview — at most 3–5 questions: who uses
   this, current pain, next 3-month goal, hard constraints (compliance,
   perf, team).
4. Draft **CTX-001** (vision + goals) and **1–3 ADRs** for
   load-bearing decisions visible in the code (framework choice, auth
   provider, deployment target, datastore).
5. Show drafts; user says ship / iterate / abort. On ship, the agent
   calls `docops new ctx` / `docops new adr` with `--body -`.
6. Close with a "now try this" handoff (see §3).

Power-user escape hatch: `/docops:onboard --auto` (or the natural
phrasing equivalent) skips the interview and drafts purely from
code-evidence. Used by users who want a starting point and will edit
freely. Documented in the cookbook chapter.

### 3. Next-step affordances on every mutating CLI command

Every command that mutates state ends with a `→ Next:` block: 1–3
concrete follow-up commands or slash-moments tailored to what just
happened. Affected verbs: `init`, `new ctx|adr|task`, `refresh`,
`amend`, `supersede`, `audit`, `state`, `upgrade`.

Format is uniform — bullet list, each line a literal command the user
can copy:

```
$ docops new adr "..."
created ADR-0033 docs/decisions/ADR-0033-...md
→ Next:
  • docops new task "..." --requires ADR-0033
  • docops refresh
  • /docops:plan ADR-0033
```

Suppression rules: `--json` output omits the block (machine
consumers); a global `--quiet` flag suppresses for scripts. Default
is on — humans benefit, scripts opt out.

Implementation is a small `internal/nextsteps` package keyed by verb
and outcome. Each verb decides its own suggestions; nothing is
auto-derived. Keeps suggestions high-signal at the cost of a
maintenance touchpoint per new verb.

## Rationale

- **Detection is a heuristic, not AI.** A lightweight file-system
  check is fast, deterministic, testable, and survives offline.
  Anything fancier is overengineering for what's effectively a routing
  decision (which next-step block to print).
- **Onboarding belongs in a skill, not a Go verb.** The work is
  scanning, interviewing, drafting — agent territory. CLI verbs stay
  thin and composable per ADR-0011 / ADR-0029. The skill calls
  existing `docops new` to write, so there is no duplicate code path.
- **Next-step affordances are the bridge.** ADR-0029 split surface
  into ops (CLI), verbs (commands), and moments (slash). Affordances
  are the missing arrow from ops back to moments — they teach the user
  the next moment without forcing them to read docs.
- **Greenfield gets the same treatment.** This is not just a
  brownfield feature. The blank-page problem is universal. Brownfield
  detection just routes greenfield users to a different opening
  prompt.
- **`--auto` keeps interviews opt-out.** Forcing 5 questions on every
  adoption irritates power users. Making the interview opt-out instead
  of opt-in keeps the default high-touch (better for the median user)
  while preserving the fast path.

## Consequences

- New skill chapter `cookbook/onboard.md` in the docops skill bundle.
  Mirrored by `docops upgrade` to all harnesses (Claude, Cursor, Codex,
  OpenCode).
- `internal/cli/init.go` gains a brownfield detector + routing logic
  for the closing message.
- New `internal/nextsteps` package + per-verb wiring across ~8
  commands. Adds a small maintenance burden: every new mutating verb
  must register a suggestion set or fall back to a generic line.
- `--quiet` global flag added to root cobra command if not already
  present.
- README "Getting started" section needs a single-paragraph rewrite
  pointing brownfield users at `/docops:onboard` and greenfield users
  at `/docops:plan`. Out of scope for this ADR — captured as a TP.
- `docops init --json` output gains a `next_steps: [...]` array so
  programmatic callers can render their own affordances.
- The onboard skill writes drafts via `docops new` — no new write
  path, no new schema. Drafts are normal CTX/ADR files; user-edits
  apply normally.

## Out of scope

- AI-assisted ADR backfill from `git log` / inline comments — a
  natural follow-up but a much larger surface. If demand emerges,
  capture as a separate ADR.
- Importer for existing `adr-tools` / MADR / `log4brains` folders.
  Same reason — separate ADR when needed.
- Linear / Jira / GitHub Issues importer. Separate problem entirely.
