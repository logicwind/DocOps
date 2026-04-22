---
title: Implement `docops audit` — structural gap detection
status: done
priority: p1
assignee: claude
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

## Outcome (2026-04-22)

- **Package:** `internal/audit` (`audit.go`, `rules.go`, `render.go`) plus `cmd/docops/cmd_audit.go`.
- **Pipeline:** `docops audit` runs `validate` first and refuses on a broken repo, then builds the index in-memory and evaluates six rules.
- **Rules:**
  - `adr-accepted-no-tasks` → **error**: ADR accepted + coverage=required, zero citing tasks (any status), age > `AdrAcceptedNoTasksAfterDays`.
  - `adr-draft-stale` → **warn**: draft older than `AdrDraftStaleDays`.
  - `task-active-stalled` → **warn**: active task with `age_days > TaskActiveNoCommitsDays`.
  - `task-cites-superseded` → severity from `cfg.Gaps.TaskRequiresSuperseded{Adr,Ctx}`, normalised to `warn` by default; `off` suppresses. Includes successor ID in the message when available.
  - `ctx-orphan` → **warn**: CTX with empty `DerivedADRs` + empty `ReferencedBy` past `CtxWithNoDerivedLinksAfterDays`.
  - `adr-coverage-review` → **info**: only when `--include-not-needed`; one finding per opt-out ADR for periodic review.
- **CLI contract:** `--json` emits `{ok, findings: […]}`, `--only <rule>` narrows, `--include-not-needed` enables the info rule. Exit 1 if any error-severity finding exists; 0 otherwise (warn/info never break the build); 2 for bootstrap errors.
- **Determinism:** sorted by severity (error → warn → info) then rule then id. Human and JSON output are byte-stable across runs on the same inputs.
- **Tests:** 30 total in `internal/audit` — every rule at-threshold / below-threshold, severity config paths (warn / error / off) for both ADR and CTX supersedes, `--include-not-needed` gating, `FilterByRule` narrowing, `HasErrors` transitions, sort order, and a dog-food pass over the live repo.
- **Dog-food:** `./bin/docops audit` on this repo reports `0 errors, 0 warnings, 0 info`. With `--include-not-needed`, it surfaces ADR-0014 (`coverage: not-needed` for the positioning-as-substrate decision) as the one info finding.
