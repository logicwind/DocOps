---
name: new-adr
description: Create a new ADR (Architecture Decision Record) for a design or process decision. Use when the user is about to write code that encodes a decision not yet recorded.
---

# /docops:new-adr

Create a new ADR under `docs/decisions/`.

Preferred pattern for agents — create and populate in one call:

```
docops new adr "Title" [--related ADR-xxxx,CTX-yyy] --body - <<'EOF'
## Context

What problem forced this decision.

## Decision

What will be done.

## Rationale

Why this option.

## Consequences

What this enables or restricts.
EOF
```

If you already have the body in a file:

```
docops new adr "Title" --body-file /path/to/body.md --json
```

`--body` and `--body-file` are mutually exclusive. Both imply `--no-open`.

An ADR captures a decision. Default fields: `status: draft`, `coverage: required`, `date: <today>`. The user may change `coverage` to `not-needed` if no task will cite this ADR — but that requires a short justification in the ADR body.

Once the ADR is accepted, at least one task should cite it. Pair this skill with `/docops:new-task` to avoid an orphaned decision.
