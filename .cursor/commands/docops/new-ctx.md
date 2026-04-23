---
name: new-ctx
description: Create a new CTX (Context) document — PRD, memo, research, design brief, constraints, or interview notes. Use when capturing stakeholder input or guardrails.
---

# /docops:new-ctx

Create a new CTX under `docs/context/`.

Preferred pattern for agents — create and populate in one call:

```
docops new ctx "Title" --type memo --body - <<'EOF'
## Summary

Capture the stakeholder input or constraint here.
EOF
```

If you already have the body in a file:

```
docops new ctx "Title" --type brief --body-file /path/to/body.md --json
```

`--body` and `--body-file` are mutually exclusive. Both imply `--no-open`.

`--type` must be one of the values listed in `docops.yaml` → `context_types`. Defaults shipped with init: `prd`, `design`, `research`, `notes`, `memo`, `spec`, `brief`. Projects may add their own types.

CTX docs capture the *why* — stakeholder intent, constraints, research. They are the raw material ADRs cite when justifying a decision. Keep each CTX focused: one PRD per feature, one memo per guardrail, one research note per investigation.
