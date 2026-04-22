---
title: Implement `docops audit` — structural gap detection
status: backlog
priority: p1
assignee: unassigned
requires: [ADR-0008, ADR-0009]
depends_on: [TP-004]
---

# Implement `docops audit` — structural gap detection

## Goal

Command that reports structural coverage gaps computed from `.index.json`. Separate from STATE.md: this is the actionable punch list, not a passive snapshot.

## Acceptance

- Runs the structural rules from ADR-0008:
  - ADR `accepted`, `coverage: required`, zero citing tasks, age > threshold → gap.
  - ADR `draft` older than threshold → stalled decision.
  - Task `active` without recent commits → stalled work.
  - Task citing a superseded ADR/CTX → stale reference (warning or error per `docops.yaml`).
  - CTX with no derived ADRs or tasks after threshold → potential orphan.
- Human-readable output groups findings by severity (error / warning / info).
- `--json` flag emits structured findings for scripting.
- `--only <rule-name>` flag limits to a specific rule.
- `--include-not-needed` includes ADRs with `coverage: not-needed` for periodic review.
- Exit code is non-zero if any `error`-level findings exist; useful in CI.

## Notes

Audit is the command agents run before starting a session (per AGENTS.md). Keep output high-signal; every finding must have an implied action.

Semantic-coverage review (`docops review`) is a separate command, not part of this task. Keep scope tight.
