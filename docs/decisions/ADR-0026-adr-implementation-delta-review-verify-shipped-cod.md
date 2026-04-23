---
title: ADR implementation delta review — verify shipped code matches decision
status: draft
coverage: required
date: "2026-04-23"
supersedes: []
related: [ADR-0010, ADR-0008, ADR-0018]
tags: []
---

## Context

DocOps derives ADR `implementation` state from citing tasks (per ADR-0010):
once every citing task is `done`, implementation flips to `done`. But "all
tasks done" is not the same as "the ADR's Decision and Consequences are
actually encoded in the shipped code." A task can ship something adjacent,
a Consequence can drift over subsequent refactors, or a Rationale can be
invalidated by downstream work. Without a review gate, ADRs accumulate as
aging contracts that nobody audits against reality.

Today the only signal we have is `implementation=done` from ADR-0010.
There is no record of *who verified that the code matches the decision,
or when*.

## Decision

Add an optional source frontmatter field to ADRs:

```yaml
reviewed_at: YYYY-MM-DD   # absent = never reviewed
```

Ship a new CLI command `docops review`:

- `docops review` (no args) — list ADRs with stale review:
  `implementation == done` AND (`reviewed_at` missing OR `reviewed_at` <
  newest commit touching any file referenced by the ADR's citing tasks).
  Supports `--json`.
- `docops review <ADR-ID>` — print the ADR body plus recent commits
  touching files referenced by its citing tasks. The agent (or human)
  reads this and decides whether code matches decision.
- `docops review <ADR-ID> --mark` — stamp today's date into `reviewed_at`
  via atomic frontmatter rewrite (not hand-editing).

Surface a `stale_review` count in `docs/STATE.md` next to other
needs-attention items, so the review backlog shows up in orientation.

## Rationale

- The computed `implementation` state (ADR-0010) is good enough to
  *trigger* a review, but the review itself is human+agent judgment —
  reading shipped code against a decision — so the sign-off belongs in
  source, not index. Keeping it as a source field means we don't try to
  infer "reviewed" from something the filesystem can't tell us.
- A dedicated command keeps this out of band: don't force review on
  every touch; let the user (or an agent running `docops review`) close
  the loop when they want to.
- Matches the CLI-as-query-API posture (ADR-0011, ADR-0018). No new data
  model primitives, just a new field and a new verb.

## Consequences

- +1 optional frontmatter field (backwards compatible via schema
  `required`/`additionalProperties`).
- +1 CLI surface; the stale-detection algorithm needs git-awareness
  (already available elsewhere — `last_touched` is computed from git).
- Gives us a lightweight mechanism to catch ADR/code drift without
  heavyweight review meetings.
- Opens a follow-on: integrate `docops review` into the agent
  orchestration skills (see TP-023 phase 2) so agents can prompt for
  review when implementation flips to done.
