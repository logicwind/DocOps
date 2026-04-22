---
title: Coverage model — structural gaps and semantic review
status: accepted
coverage: required
date: 2026-04-22
supersedes: []
related: [ADR-0004, ADR-0007, ADR-0009]
tags: [coverage, audit, workflow]
---

# Coverage model — structural gaps and semantic review

## Context

Not every ADR will have tasks created for it promptly; not every CTX will translate into an ADR. Without a forcing function, gaps accumulate silently and "implementation: accepted but nothing built" becomes the norm. An LLM working in the repo should be able to spot and close gaps.

## Decision

DocOps treats coverage as two distinct problems with different fixes.

### Structural coverage (mechanical)

Detected by the indexer from the graph alone. No LLM judgment needed. Examples:

- ADR with `coverage: required` and `status: accepted` but zero citing tasks → gap.
- CTX with no derived ADRs and no citing tasks after N days → possible orphan.
- Task citing a superseded ADR/CTX → stale reference.
- Task `active` > N days with no commits → stalled.

Surfaced in:
- `docs/STATE.md` under "Needs attention."
- `docops audit` command output (CLI + `--json`).

### Semantic coverage (LLM judgment)

Structural rules cannot tell whether existing tasks *actually cover* an ADR's intent. This requires reading the ADR body and the citing tasks' acceptance criteria. The workflow:

1. An agent (or human) runs `docops review <ADR-id>`.
2. The CLI bundles: ADR body, every task citing the ADR (title, status, body), motivating CTX docs, any prior reviews.
3. The agent returns a structured verdict written to a sidecar: `docs/decisions/.reviews/<ADR-id>.yaml`.
4. The sidecar contains: `reviewed_at`, `reviewed_by`, `verdict` (`covered | partial | uncovered`), `gaps`, `proposed_tasks`.

Indexer reads sidecars and exposes `last_reviewed_at` and `last_verdict` in the index. STATE.md surfaces ADRs not reviewed in N days or with a `partial`/`uncovered` verdict.

## Rationale

- Splitting mechanical detection from judgment lets tooling do 80% of the work and agents do the remaining 20%.
- Sidecars keep review provenance in git without polluting the ADR source frontmatter.
- Configurable thresholds in `docops.yaml` let projects tune signal.

## Consequences

- Review sidecars ARE committed to git (audit trail).
- `docops review` is a phase-1 command but its output format may evolve; keep backward compatibility with `reviewed_at` and `verdict` fields.
- Agents performing reviews should always record who reviewed and what model; gives humans a way to trust but verify.
- Review sidecars may be created for CTX in the future; phase 1 limits them to ADRs.
