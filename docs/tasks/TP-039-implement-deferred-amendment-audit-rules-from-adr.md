---
title: Implement deferred amendment audit rules from ADR-0025
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0025]
depends_on: []
---

## Goal

Implement the three audit rules ADR-0025 §"Audit rules" specified but
that TP-026 phase 3 deferred:

1. **Drift threshold** — flag ADRs with ≥ 5 amendments (configurable),
   suggesting a superseding ADR may be cleaner than continued amending.
2. **Hand-edit drift** — flag ADRs whose body changed in git without a
   matching frontmatter `amendments[]` entry in the same commit. Catches
   silent edits that should have gone through `docops amend`.
3. **Stale-ref** — flag amendments whose `ref:` is a task id where the
   task is still `backlog` and the amendment is older than
   `gaps.amendment_stale_days`. Means "we said we'd do X; X still isn't
   done."

## Acceptance

- New `audit` rules emit findings with the standard `id / reason / next`
  shape, gated on `gaps.*` config keys with sensible defaults
  (e.g. `amendment_drift_threshold: 5`,
  `amendment_stale_days: 90`).
- Hand-edit drift uses `git log -p` over the ADR file path; rule is
  best-effort (skip silently if not in a git repo, like other audit rules
  that touch git).
- Tests cover: clean repo (no findings), each of the three rules
  triggering, and config overrides.
- ADR-0025 §"Audit rules" cited in commit message; this TP closes the
  open work item from there.

## Notes

Was deferred from TP-026 phase 3 (commit 03bcaea) — the data layer + CLI
+ index/STATE shipped first; audit is additive surfacing on top.
