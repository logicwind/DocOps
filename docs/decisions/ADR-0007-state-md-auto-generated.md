---
title: STATE.md — auto-generated project snapshot
status: accepted
coverage: required
date: 2026-04-22
supersedes: []
related: [ADR-0005, ADR-0008]
tags: [index, ui, dashboard]
---

# STATE.md — auto-generated project snapshot

## Context

Agents landing cold in a DocOps repo need a compact, high-signal summary of current state before they start loading individual docs. `.index.json` is machine-optimal; humans and LLMs both also want a readable overview.

## Decision

`docops index` (and the pre-commit hook) generates `docs/STATE.md` as a projection of `.index.json`. The file is regenerated on every index run and should never be hand-edited.

Required sections, in order:

1. **Header** — generation timestamp.
2. **Counts** — N by status, for each of CTX / ADR / Task.
3. **Needs attention** — actionable items sorted by severity (see below).
4. **Active work** — tasks with `status: active`, formatted `TP-XXX (assignee, priority) title — requires: ...`.
5. **Recent activity** — last N doc changes within a configurable window.

The "Needs attention" section is the load-bearing one. Each bullet represents a rule that produces an actionable item. Rules are declared in `docops.yaml` under a `gaps:` key; thresholds are tunable. Standard rules:

- ADR `accepted` with `coverage: required`, zero citing tasks, age > N days.
- ADR `draft` with age > N days.
- Task `active` with no commits in N days.
- Task citing a superseded ADR or CTX.
- CTX with zero derived ADRs or tasks after N days.

## Rationale

- Markdown is readable by humans and by every LLM without special parsing.
- Generating from the single `.index.json` source avoids drift between JSON and MD views.
- Keeping "Needs attention" configurable lets small projects tune signal/noise.

## Consequences

- STATE.md is committed to git (same rationale as `.index.json` — offline readability for CI and fresh agents).
- It must never be manually edited; a pre-commit hook regenerates it and refuses conflicting manual edits.
- Adding new rule categories is a config change in `docops.yaml`, not a code change. Built-in rules are in the CLI; custom rules can be added later.
