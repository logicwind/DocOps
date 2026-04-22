---
name: new-adr
description: Create a new ADR (Architecture Decision Record) for a design or process decision. Use when the user is about to write code that encodes a decision not yet recorded.
---

# /docops:new-adr

Create a new ADR under `docs/decisions/`.

```
docops new adr "<title>" [--related ADR-xxxx,CTX-yyy]
```

An ADR captures a decision. A good ADR includes:

- **Context** — what problem forced the decision.
- **Decision** — what will be done (imperative, one paragraph).
- **Rationale** — why this option, not the alternatives.
- **Consequences** — what this enables, restricts, or requires downstream.

Default fields: `status: draft`, `coverage: required`, `date: <today>`. The user may change `coverage` to `not-needed` if no task will cite this ADR — but that requires a short justification in the ADR body.

Once the ADR is accepted, at least one task should cite it. Pair this skill with `/docops:new-task` to avoid an orphaned decision.
