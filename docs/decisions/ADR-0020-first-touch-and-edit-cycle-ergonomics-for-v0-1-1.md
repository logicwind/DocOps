---
title: First-touch and edit-cycle ergonomics for v0.1.1
status: accepted
coverage: required
date: 2026-04-22
supersedes: []
related: [ADR-0004, ADR-0011, ADR-0013, ADR-0015, ADR-0018]
tags: [cli, ergonomics, agent-interface, v0.1.1]
---

# First-touch and edit-cycle ergonomics for v0.1.1

## Context

First-user testing of v0.1.0 on a real project (`lw-website-nextjs-web`)
surfaced three distinct friction points that are not bugs but are
design regressions relative to what a polished CLI would do:

1. **`docops init` is surprising.** It runs in the current directory
   with no announcement, no confirmation, and no way to point at a
   different folder. The user's reaction was "Setup in the folder —
   should ask (and explain what it will do, does other project do
   this?)". Comparable tools land on different sides of this: `git
   init` silently runs in cwd and accepts `[dir]`; `npm init`, `cargo
   generate`, and `create-next-app` all confirm interactively.

2. **The edit loop is a 4-command chain.** Every time a doc edit
   happens, the agent or human runs
   `docops validate && docops index && docops state && docops audit`.
   The test transcript shows the AI agent chaining three of those in a
   single Bash call. That is 3× the commands it should need.

3. **Agents cannot create-and-populate in one step.** The Claude Code
   `Read-Before-Edit` hook refuses `Write` on a file that was not
   first read via the `Read` tool. `docops new` creates a file; the
   agent then tries to `Write` its real content; the hook rejects.
   The workaround (a `Read` call per file) is purely ceremonial friction
   that DocOps caused by emitting a stub body agents discard anyway.

In addition, when the same test hit a bad enum value (`status:
proposed` on an ADR — the real values are `draft|accepted|superseded`),
the validator rejected it correctly but did not surface the allowed
set in the error message, costing another round-trip for the agent to
discover the right value.

None of these were caught in phase-1 design because the CLI was
self-tested by its own author; agents driving it cold is the real
workload.

## Decision

Three CLI surface changes, shipped together as v0.1.1. Each has its
own task (TP-015, TP-016, TP-017) citing this ADR. A small validator
error-message fix ships inside TP-017.

### 1. `docops init [dir]` — announce, confirm, and accept a path

- **Positional `[dir]`** — when present, init targets that directory
  (created if absent) instead of cwd. Matches `git init` muscle memory.
- **Action announcement** — before any prompt, print a two-line
  header: what init does (create dirs, write config, schemas, git
  hook, skills) and that re-running is safe. Follow it with the
  existing "+ …" plan list.
- **Interactive confirm by default on a TTY** — prompt `Proceed? [y/N]`
  after the plan. Skip the prompt when stdin is not a terminal, when
  `--yes` / `-y` is passed, or when `--dry-run` is passed.
- **`--yes` flag** — non-interactive force, same semantics as "y" at
  the prompt. Shell scripts and CI pass this.

Non-decisions: `init` still runs in cwd when no arg is given (no
breaking change); `--force` and `--no-skills` semantics are unchanged;
piping (`echo y | docops init`) works because the confirm-skip path
fires when stdin is not a TTY.

### 2. `docops refresh` — the edit-loop collapse

A new subcommand that runs `validate → index → state` in one pass and
exits non-zero if any sub-step fails. `audit` stays out of the
sequence — it is advisory, not a refresh operation, and should not
gate committing doc edits on subjective findings like "ADR is 8 days
old".

- Human output: one line per sub-step plus a final
  `docops refresh: OK`.
- Exit codes mirror the underlying commands (validate failure → 1,
  bootstrap error → 2).
- `--json` returns the aggregate status plus the per-step summary
  shape so CI consumers can branch.

The pre-commit hook shipped by init stays `docops validate`-only for
v0.1.1. Auto-regenerating STATE.md and .index.json from the hook is
tempting but introduces awkward staging semantics (either the hook
silently `git add`s the regenerated files — magic — or it errors out
asking the user to stage them, producing a worse UX than today).
That decision is deferred; `refresh` is the 80% win.

### 3. `docops new <kind> --body` and `--body-file` — agent-friendly composition

- **`--body -`** — read the document body (everything after the
  frontmatter) from stdin. Replaces the default stub entirely.
- **`--body-file <path>`** — read the body from a file instead.
- Mutually exclusive with each other and with `--no-open` gets
  ignored when `--body` is used (body is already populated; no reason
  to open `$EDITOR`).
- When neither flag is present, behaviour is unchanged: stub body,
  open `$EDITOR` unless `--no-open`.

This is one `cmd_new.go` flag + one `internal/newdoc` Option. It
removes the Claude Code Read-Before-Edit friction entirely because
the agent never has to re-open the file to write its content.

### 4. Validator enum-error messages (under TP-017)

Every `invalid-enum` finding from the validator includes the list of
allowed values in the human-readable `Message`. Today the message
says something like "invalid status value"; the improved form reads
"status \"proposed\" is not one of: draft, accepted, superseded".
Agents fix the value on first read instead of guessing.

## Rationale

- The three CLI changes are individually small (a flag, a new
  subcommand, a prompt) and together halve the number of commands an
  agent has to issue in a realistic DocOps session.
- Confirm-by-default keeps first-touch safe without blocking
  scripted adoption (TTY detection + `--yes` are the same escape
  hatch every mature CLI uses).
- `refresh` avoids introducing new logic — it is pure composition over
  already-shipped commands. Low risk, high payoff.
- `--body -` reflects ADR-0018's stance that the CLI *is* the query
  layer; extending that reasoning, the CLI should also be the
  creation layer end-to-end, not a scaffolding step that forces
  external editors to finish the work.
- Validator message improvement is a one-liner in the finding
  formatter; deferring it would be cargo-culting.

## Consequences

- Interactive `init` now blocks briefly on TTY. Scripts must pass
  `--yes`, which becomes the documented install-in-CI pattern.
- `docops refresh` is a composite; if a future task wants to run the
  three underlying steps in a different order or with different
  flags, they still call the primitives directly.
- `--body` and `--body-file` on `docops new` accept user-supplied
  content. The body is written verbatim; it is the user's
  responsibility to keep frontmatter out of it (docops prepends its
  own frontmatter). A leading `---` in `--body` content is treated as
  body (not reparsed as frontmatter) — document this in the flag help.
- The pre-commit hook stays simple. Users who want auto-refresh on
  commit can add `docops refresh` to their own hooks; DocOps does
  not impose it.
- The validator enum-hint change is observable by callers who
  string-match the message. Our own tests that do string matching
  must update; external consumers must not rely on the message shape
  (the `Rule` field is the stable contract, per TP-003).
