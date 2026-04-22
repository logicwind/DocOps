# Thinking Doc — GitOps-Native SDLC for LLM-Agentic Projects

> **Reading material for your flight.** This is not a spec. It's a
> structured tour through the ideas, vocabulary, and reading that should
> let you land with a crisp decision set for `sdlc-app2`.
>
> You already proved the core thesis on the Zund repo: **ADR-driven
> development with an LLM pair worked — you shipped, the decisions are
> legible, nothing important lives only in chat.** This doc is about
> generalizing that into a tool other teams can use, and the decisions
> that puts in front of you.

---

## 0 · How to read this

Pick the sections by why you opened it:

- **Want strategy?** → §1 (problem), §10 (competition), §12 (open questions)
- **Want vocabulary?** → §2 (restated core), §4 (ADRs), §5 (tasks)
- **Want the hard parts?** → §8 (GitOps patterns), §9 (design questions)
- **Want to read more after?** → §11 (reading list)

You can skip around; sections are independent.

---

## 1 · The problem you are replacing

### 1a. JIRA fatigue

For small-and-medium projects, JIRA (and Shortcut, Linear-at-scale,
Asana, etc.) is an *external database of project truth* that:

- Lives outside the repo. Changes to the repo and changes to tickets
  drift apart silently.
- Is owned by whoever remembers to update it. Two weeks in, ticket
  states lie.
- Requires every contributor to have an account, permissions, a
  license. PM-friendly, dev-hostile.
- Gets abandoned first when the team is under pressure.
- Is opaque to LLMs. An AI pair can read code and commits; it cannot
  natively read your JIRA without an integration.

### 1b. Confluence/Notion docs rot

Docs-in-wiki have a matching failure mode:

- No PR review on docs → no one catches drift.
- No git blame → "who wrote this and why?" is unanswerable.
- No diff → what changed between last quarter and now? Unknown.
- Not testable — can't validate that the claims are still true.
- Not in the AI's context by default.

### 1c. The observation behind `sdlc-app2`

You don't need a better ticketing tool. You need the repo to **be**
the project database, and a thin UI that lets non-technical
collaborators join without learning git.

For small-to-medium projects — a founder + 3 engineers + a designer +
a PM; an OSS project with 6 maintainers; an internal tool team of 10 —
this is enough. You don't need Jira's 400 fields. You need:

- A durable record of *why we decided things* (ADRs).
- A lightweight record of *what we're doing next* (tasks, roadmap).
- A record of *how it used to be* (archive).
- A way for non-git people to see and contribute safely.

And critically: **the LLM agent can read all of it natively**, because
it's just files in the repo it is already working in.

---

## 2 · The core idea, restated

Your five principles, with three additions from the first pass:

| # | Principle | One-liner |
|---|---|---|
| 1 | Filesystem IS the database | Markdown + YAML frontmatter. No external DB. |
| 2 | Git IS the backend | Commits are changes. History is `git log`. Diffs are updates. |
| 3 | Frontmatter IS the schema | Every file conforms to a type. Invalid = rejected pre-commit. |
| 4 | Web app IS the non-dev interface | Bun/React viewer; PMs/clients browse without a terminal. |
| 5 | Commits ARE submissions | Web edits write files + commit. No separate API. |
| 6 | Folder IS the type | `/adrs`, `/tasks`, `/docs`, `/sprints`. Schema per folder. |
| 7 | Board IS a saved query | Kanban views are filter+groupby yaml files. |
| 8 | Activity feed IS `git log` | Audit, cycle time, velocity — all derived. |

**The claim:** with these eight, you have ~85% of JIRA-for-small-teams
without any server, any database, or any account system beyond what
your git host already provides.

---

## 3 · Why this matters for LLM-agentic development

This is the wedge you actually have. Not "git-native tickets" — a
dozen tools have tried that. The wedge is:

**LLM coding agents are productive in direct proportion to how much
project context lives next to the code.**

Observations from your Zund run:

- ADRs let you stop re-explaining the same decision to the LLM every
  session. The decision is a file it can read.
- Implementation status (`implementation: not-started | partial |
  done`) lets the agent answer "is this real yet?" without guessing.
- The archive preserves context the code can't show — *why we chose
  this shape over that one*. Invaluable for the agent's judgment on
  refactors.
- The `roadmap/current.md` + `next.md` pattern gives the agent "what
  are we doing right now?" without you re-briefing each morning.

JIRA can't participate in this. Its API is slow, its auth is a
headache, and its data model is orthogonal to what a code-reading
agent needs. Repo-native docs are the shape the agent wants.

**The slogan:** *Your project management tool should be a library the
LLM reads, not a SaaS it can't reach.*

---

## 4 · ADRs — the heart of it

### 4a. What is an ADR?

An **Architecture Decision Record** is a short, dated, immutable
document that captures a single architectural choice: the context, the
decision, and the consequences.

The format is 13 years old; Michael Nygard named and described it in
2011. It has become the dominant pattern for keeping engineering
decisions legible over time.

### 4b. Why immutable?

A decision record is *history*. Mutating it destroys the audit trail.
If circumstances change, you write a **new** ADR that *supersedes* the
old one; the old one gets `status: superseded` and a pointer forward.

This is not bureaucracy — it's the property that makes the file useful.
You can ask, six months later, "what did we know and believe when we
chose X?" and get an honest answer.

### 4c. Anatomy of a good ADR

Minimal structure (Nygard's original):

```
# NNNN · Title

Date: YYYY-MM-DD
Status: proposed | accepted | superseded | rejected

## Context
What is the situation forcing a choice? What constraints apply?

## Decision
What have we decided? State it directly.

## Consequences
What becomes easier? Harder? What new risks or obligations?
```

Your Zund variant adds two fields that I think are load-bearing:

- **`implementation`** (not-started / partial / done / n/a) —
  decision state is *orthogonal* to code reality. "Accepted but
  not-started" is a valid, useful state: "we've decided, haven't
  built."
- **`supersedes` / `superseded_by` / `related`** — the graph of how
  decisions connect. This is how `decisions/README.md` renders a
  legible history instead of a timeline.

These two additions are more than cosmetic. They turn the ADR folder
from a *log* into a *state machine the LLM can reason over*.

### 4d. When to write an ADR vs. not

An ADR is the right tool when the decision is:

- **Architectural** — affects the shape of the system, not the color.
- **Expensive to reverse** — changing it later would cost weeks.
- **Cross-cutting** — multiple components are affected.
- **Non-obvious** — future-you or a new contributor would reasonably
  wonder "why did we do it this way?".

It is the wrong tool for:

- Tactical code changes ("rename this function").
- Style preferences that can live in a linter config.
- Hypothetical decisions that aren't forced by a real constraint.

A good heuristic: **if you find yourself explaining the same choice to
the LLM three times, write an ADR.**

### 4e. ADR variants worth knowing

- **Nygard format** — the original. Context / Decision / Consequences.
- **MADR** (Markdown Any Decision Records) — a more structured
  template with explicit options-considered sections. Good when the
  alternatives matter. See §11.
- **Y-statement** — one-paragraph form: *"In the context of X, facing
  Y, we decided Z to achieve W, accepting trade-off V."* Useful for
  quick decisions; expand to full ADR if the decision has legs.
- **Lightweight ADR** — just a commit message with a conventional
  prefix. Fine for small teams; doesn't scale to "I want to see all
  security-related decisions from the past year."

Your project uses a Nygard-ish format with typed frontmatter — that
is the right choice for this project.

### 4f. Patterns you are already running (keep these)

- **Single source of truth for the index is the frontmatter.** The
  README.md is regenerated. Humans don't hand-edit it.
- **Pre-commit hook validates.** Broken references, duplicate IDs,
  invalid statuses fail the commit. Cheap, fast, high signal.
- **Tags are free-form but faceted.** Let them grow organically.
  Once a tag has 5+ uses, consider promoting it to a folder or
  canonical label.

---

## 5 · Tasks — lightweight, living in the repo

### 5a. Why task-as-file beats task-as-row

A task in JIRA is a row in a database with 40 optional columns. A
task as a `.md` file is:

- Diffable — the history of the task is `git log`.
- Referenceable — PRs can link `Closes TASK-042`.
- Reviewable — you can put a task change in a PR and have someone
  review it, just like code.
- Attachable — screenshots, logs, and decision sketches live next to
  the task in the same folder.
- Readable by the LLM — no API call, no auth, no rate limit.

The trade-off is that some things JIRA does gracefully (board views,
burndown charts, sprint planning) require derivation rather than
being built-in. That's §7 and §8.

### 5b. Minimum useful task schema

Your current task schema is already good:

```yaml
id: "task-001"
title: "..."
type: "task"
status: "pending"       # pending | active | blocked | review | done | cancelled
owner: "pi"             # person, team, or agent
priority: "high"        # low | medium | high | critical
due_date: 2026-05-01    # optional
dependencies: []        # list of task ids
tags: [...]
description: "..."      # one-liner summary
```

Two things to consider adding:

- **`epic`** or **`parent`** — a pointer to a bigger work item. Lets
  you group tasks under a roadmap slice without a separate concept.
- **`pr` or `commits`** — once a PR is linked, the task can trace its
  own implementation. You don't have to maintain this by hand if the
  CI parses PR bodies for `Closes TASK-XXX`.

### 5c. Where tasks belong vs. other tools

Your `docs/index.md` already takes a stance on this:

| Scope | Where |
|---|---|
| Line-level | `// TODO:` in code |
| Task-level | `docs/tasks/` (or GitHub Issues) |
| Slice-level | `docs/roadmap/current.md` |
| Someday | `docs/roadmap/next.md` or `docs/braindump.md` |

That taxonomy is right. The only thing worth re-examining: for
tickets you expect non-technical people to edit, the friction of
GitHub Issues is often lower than a repo file — until you have the
web UI. After you have the web UI, the file wins.

### 5d. The boring parts (sprints, boards, assignment)

- **Sprint** = a document at `docs/sprints/2026-W17.md` with a list
  of tasks, or just a `sprint:` field on each task. Either works;
  the latter is simpler.
- **Board** = a saved query. `.boards/mine.yaml` with
  `filter: owner=nix AND status!=done` and `groupBy: status`. The
  web app renders Kanban from that.
- **Assignment** = editing `owner` in frontmatter. That's a commit.
  The commit body has a message. `git log` tells you who has been
  assigned what, when, and by whom.

None of this needs new infrastructure.

### 5e. The anti-pattern: checklists that rot

`docs/index.md` has the right rule: *"markdown TODO lists rot
silently."* Don't keep a parallel checklist of "what to do this
week" inside a doc. Either:

- It's a task → it's a file with a status.
- It's a thought → it goes in braindump.
- It's a line-level TODO → put a `// TODO:` in the code.

Nowhere else.

---

## 6 · Docs proper — reference / roadmap / archive

This is the taxonomy you already have. Worth restating because it's
the bit most teams get wrong.

- **`reference/`** — *how it is.* Stable. If code changes, docs
  change in the same PR. Stale reference is a bug.
- **`roadmap/`** — *where we're going.* Forward-looking. Only one
  `current.md`. Shipped plans move to archive.
- **`archive/`** — *how it was.* Write-once, never update. Preserves
  the "why" the code can't show.

A useful further reading: **Diátaxis** — a framework that splits all
docs into Tutorials / How-To / Reference / Explanation. Your
3-bucket split collapses those usefully for a small team; if the
project grows, splitting `reference/` into `reference/` (facts) and
`explanation/` (concepts) is the natural next step. See §11.

---

## 7 · Frontmatter as schema

Frontmatter is YAML at the top of the file, fenced by `---`. It's
both human-readable and machine-parseable. That dual nature is what
makes this system work.

### 7a. What makes a good schema

- **Required fields, few.** `id`, `title`, `type`, `status`,
  `last_updated`. Beyond that, add only when a field earns its keep.
- **Enums, not free text, for state.** `status: done` is checkable.
  `status: "done-ish, waiting on QA"` is not.
- **Typed relations.** `related: [0003, 0005]` with a validator that
  checks the IDs exist. Otherwise, links rot.
- **Dates, not "recent".** `last_updated: 2026-04-21` beats "a while
  back."
- **Tags are free-form escape hatch.** Don't over-structure; let
  patterns emerge, then promote.

### 7b. The validator is the contract

The pre-commit hook is what makes the schema real. Without it,
frontmatter drifts in a week. With it:

- Invalid frontmatter = rejected commit.
- Missing required fields = rejected commit.
- Broken cross-references (superseded_by pointing to nonexistent
  ADR) = rejected commit.

Three layers of validation that share one schema definition:

1. **Editor schema hints** (JSON Schema in `.schemas/`) → IDE
   autocomplete.
2. **Pre-commit** — fast, local, before the commit lands.
3. **CI** — full cross-repo validation (broken links, orphaned
   references, graph integrity).

### 7c. Your key insight: decision state vs. implementation state

Most ADR systems conflate "decided" and "built." You split them:

- `status` = *did we agree?* (proposed / accepted / superseded / rejected)
- `implementation` = *is it real?* (not-started / partial / done / n/a)

This is the single most valuable schema choice in the current setup.
It gives you four useful quadrants:

| | not-started | partial | done |
|---|---|---|---|
| **accepted** | committed but unbuilt — a promise | in-flight work | real |
| **proposed** | being debated, nothing to run | prototype exists | ? (flip to accepted) |

And it makes the README dashboard genuinely informative instead of
a list of TODOs.

---

## 8 · GitOps patterns & trade-offs

### 8a. Commit-as-submission, the upside

- Every change is reviewed. Every change has an author. Every change
  has a reason (if you write decent commit messages).
- Rollback is `git revert`. Audit is `git log`. Search is `git
  grep`. All free.
- Branches are draft state. You can prepare a plan change on a
  branch, open a PR, let people comment, then merge.

### 8b. Commit-as-submission, the downside

The hardest UX question in this whole design:

> **A PM drags a card across a Kanban board. What happens?**

Three options, with trade-offs:

1. **Every drag = one commit.** Clean audit. Noisy log. Looks
   strange in `git log` when 40 commits in one day are all
   `status: todo → in_progress`.
2. **Session-debounced commit.** The web app accumulates changes for
   N seconds/minutes, then commits with a summary message. Quieter
   log. But if two users are editing the same ticket, the merge
   gets tricky, and "who changed X, when" becomes fuzzy.
3. **Branch + PR per edit.** A drag opens a PR. Most edits
   auto-merge. This is closest to GitHub's own model. Safest,
   slowest-feeling.

**My recommendation for v1:** option 1 for small teams, with a
"squash commits on this day" cron for cleanliness. Keep it simple.
Revisit when you have concurrent-edit pain.

### 8c. Concurrency — the YAML merge conflict problem

When two users edit the same ticket's frontmatter simultaneously,
YAML merge conflicts are user-hostile. Options:

- **Last-write-wins on flat fields.** For `status`, `owner`, simple
  enum fields, LWW is fine. Most edits don't collide.
- **Field-level merge via JSON ops.** Client sends "set status=done"
  instead of a new YAML file. Server applies → commits. Avoids line
  conflicts. Needs an on-server merger.
- **Serialized commit queue.** All web-app writes go through one
  process that serializes. Simple, slightly blocks concurrency.
- **Lock while editing.** Show "Alice is editing" à la Google Docs.
  Most explicit; most UX work.

For SMB teams, **serialized queue + LWW** is overwhelmingly the best
trade. You will never see meaningful contention in a 10-person team.

### 8d. Performance — when `git log` gets slow

At ~10,000 docs, walking `git log` per page view is slow. Add a
**SQLite read-cache, rebuildable from git:**

- Post-commit hook updates the cache.
- `rm cache.db && restart` rebuilds from scratch.
- Web app reads from cache; writes go to the files (and then cache
  is updated).

**Principle preserved:** the cache is derived. Git is truth.

### 8e. Real-time

Do you need websockets + presence like Linear?

- For 2–5 person teams: no. Polling every 10s is fine. Even 30s.
- For 10+: a file-watcher → SSE stream is cheap and feels live.

Start without. Add when someone asks for it.

---

## 9 · The design questions you will decide after landing

Copy-paste these to a notes app. You need answers before building.

1. **Team-size target for v1.** Solo+PM? 5-person? 20? Decides
   concurrency, permissions, UX density.
2. **Hosting model.** Self-hosted pointed at your repo? Or hosted
   SaaS with BYO-git? Completely changes the stack.
3. **Edit model.** Commit-per-edit, debounced, or PR-per-edit?
   (See §8b.)
4. **Concurrency strategy.** LWW? Serialized queue? Lock? (See §8c.)
5. **Schema openness.** Ship with ADR/Task/Doc only, or let users
   register custom types from day 1?
6. **Notifications in v1.** Slack + email mandatory? Or read-the-feed
   only, integrations later?
7. **Auth model.** GitHub/GitLab OAuth only (simple)? Or add local
   users for clients who don't have git accounts (realistic)?
8. **Non-git contributor flow.** A client wants to comment on a task.
   Do they sign in? Comment via email? Via an un-authed link?
9. **The wedge.** Is the lead story (a) data portability, (b)
   LLM-agent-ready, (c) self-host/air-gap, (d) "your process is a
   PR"? Pick one. Say it in every landing page headline.
10. **Reference project.** Dogfood on `sdlc-app2` itself and on
    Zund? Start the demo screencast with *"this tool's own tickets
    live in the repo you're looking at."*

---

## 10 · Competition & positioning

- **Linear** — beautiful, fast, opinionated. Cloud-only. Data is
  theirs. Dev wedge, not a docs tool.
- **Notion** — great editing; no schema discipline; docs rot fast;
  not LLM-readable without integrations.
- **GitHub Issues + Projects** — free-ish, git-proximate, but:
  schema-less, boards are clunky, ADRs don't feel native, PM
  ergonomics weak.
- **Backstage** — enterprise catalog; heavy; not aimed at SMB.
- **GitBook / Docusaurus** — docs only, no ticket model.
- **Obsidian** — personal knowledge, not team, no schema enforcement.
- **Jira** — what you're replacing.

**Where you win:** the combination no one has —
- Repo-local (portable + offline + LLM-native),
- Schema-enforced (docs don't rot),
- Decisions + tasks + docs in one model (ADRs are first-class),
- Non-technical UI on top (PMs don't need git),
- Self-hostable and SaaS-able.

**Where you lose if you don't watch it:**
- Feels inferior to Linear for teams that already live in Linear and
  don't care about portability. Don't chase those.
- Network effects of hosted tools (integrations, marketplaces). Start
  with MCP/Slack/GitHub; don't try to be everything.

---

## 11 · Reading list (curated, flight-friendly)

### ADR canon

- **Michael Nygard, "Documenting Architecture Decisions" (2011)** —
  the original post. Defines the Context / Decision / Consequences
  pattern. Still the best 10-minute read on the topic. (Search by
  title; the post has moved hosts a few times.)
- **adr.github.io** — community hub. Has templates, tooling links,
  and a catalogue of ADR variants.
- **Joel Parker Henderson's ADR templates on GitHub** — the most
  comprehensive collection of ADR templates, including MADR, Nygard,
  Tyree & Akerman, and Alexandrian forms.
- **MADR (Markdown Any Decision Records)** — a more structured ADR
  format. Mandatory fields and explicit "options considered." Useful
  when alternatives matter.

### Docs philosophy

- **Diátaxis (diataxis.fr)** — the four-quadrant docs framework:
  Tutorial / How-To / Reference / Explanation. Daniele Procida.
  Worth understanding even if you keep your 3-bucket split — tells
  you what each of `reference/`, `guides/`, and archive are *for*.
- **Tom Preston-Werner, "Readme Driven Development" (2010)** —
  write the README first. Adjacent idea: your ADR is the README for
  a decision.
- **Simon Willison, "The perfect commit" and his work on
  `simonwillison.net`** — many short posts on keeping technical
  decisions and notes as durable, searchable artifacts.

### GitOps + docs-as-code

- **Weaveworks, "Guide to GitOps"** — GitOps for infrastructure, but
  the primitives (declarative, git-as-truth, reconciliation) map
  directly onto docs-as-data.
- **"Docs like code" / "write the docs" community** — long-running
  movement; search `writethedocs.org` for primers.

### LLM-agentic development

- **Claude Code & Cursor docs** — both emphasize "put context next
  to code." Your ADRs are the canonical example.
- **Anthropic's "context engineering" write-ups** — short, current,
  concrete.
- **The AutoGPT / OpenHands / Claude Agents communities** — what
  patterns work for multi-step agent workflows, and why structured
  task definitions matter.

### Complementary reading (when you have time)

- **"Team Topologies" by Skelton & Pais** — how teams interact shapes
  what documentation they need. Relevant if you're pitching this to
  bigger shops later.
- **"Accelerate" by Forsgren, Humble & Kim** — the DORA metrics
  book. The reason cycle-time and deployment-frequency graphs
  matter. Useful framing for "what reports should the SDLC app
  auto-generate?"

---

## 12 · Key insights worth internalizing

1. **Decision state ≠ implementation state.** The two-axis view is
   the single most useful bit of schema in your current system.
2. **Immutability is a feature, not a constraint.** Superseding >
   editing. Always. The archive is what makes the whole thing
   trustworthy.
3. **Index files should be generated, never hand-edited.** The
   frontmatter is the truth; the README is a view.
4. **The LLM agent is a first-class user of these docs.** Design
   the schema with "can the agent parse and act on this?" as a
   question, not an afterthought.
5. **Three buckets is enough.** Reference / Roadmap / Archive.
   Resist adding a fourth until you genuinely can't fit something
   in the three.
6. **Braindump is load-bearing.** Pre-committed thinking needs a
   home, or it leaks into ADRs prematurely. Keep it; delete
   entries when they graduate.
7. **The 5-line commit message on an ADR commit is worth more than
   the entire ADR body five years later.** Teach the team to write
   real commit messages.
8. **Hooks are cheap; use them.** `adr-index.ts` on pre-commit.
   Schema validator on pre-commit. Broken-link check on CI.
9. **The tool isn't the innovation; the discipline is.** You can
   run this with zero tooling and a shared convention. The tool
   makes the discipline survive contact with a PM.
10. **Start with Zund as the reference project.** Eat your own dog
    food. When it breaks, you'll feel it first.

---

## 13 · One-year-out vision (optional, strategic)

If this works:

- Small teams replace JIRA + Confluence with `sdlc-app2` pointing at
  a `docs/` folder. Self-hosted, offline-capable, LLM-friendly.
- A pack/plugin ecosystem forms: an ADR reviewer agent, a task
  refiner agent, a weekly-digest agent. Each is a thin hook that
  reads the repo and writes a PR.
- Clients get read-only signed URLs to view project status without
  GitHub accounts.
- The demo script: *"I'm going to show you a project-management
  tool. Here's the 'database.'"* [cat .md file] *"Here's the
  'activity feed.'"* [git log] *"Here's the 'ticket update.'"*
  [PR diff]. Zero magic; that's the sales line.

If it doesn't:

- You still have a disciplined docs system for Zund and future
  projects. That alone has paid for itself in LLM pair-programming
  productivity.

Either way, the bet is good.

---

## 14 · Before-you-land checklist

Return to this when you're about to deplane:

- [ ] Pick the target team size for v1 (§9.1).
- [ ] Pick the hosting model (§9.2).
- [ ] Decide the edit model (§9.3) — my pick: commit-per-edit.
- [ ] Decide the wedge one-liner (§9.9).
- [ ] Identify the one killer demo (see §13).
- [ ] List three non-technical people who would use v1 (real names).

Bring those six answers to your next Claude session and we can plan
the first slice.

Safe flight.
