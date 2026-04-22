---
name: new-ctx
description: Create a new CTX (Context) document — PRD, memo, research, design brief, constraints, or interview notes. Use when capturing stakeholder input or guardrails.
---

# /docops:new-ctx

Create a new CTX under `docs/context/`.

```
docops new ctx "<title>" --type <type>
```

`<type>` must be one of the values listed in `docops.yaml` → `context_types`. Defaults shipped with init: `prd`, `design`, `research`, `notes`, `memo`, `spec`, `brief`. Projects may add their own types.

CTX docs capture the *why* — stakeholder intent, constraints, research. They are the raw material ADRs cite when justifying a decision. Keep each CTX focused: one PRD per feature, one memo per guardrail, one research note per investigation.
