---
name: new-task
description: Create a new DocOps task with required ADR/CTX citations. Use when the user says "add a task" or after discovering a gap that needs a follow-up.
---

# /docops:new-task

Create a new task file under `docs/tasks/`.

Every task must cite ≥1 ADR or CTX in `requires:` (validator enforces this — ADR-0004).

```
docops new task "<title>" --requires ADR-xxxx[,CTX-yyy]
```

If the user cannot name a citation, stop and help them find or write one:

- Structural decision? Draft an ADR first via `/docops:new-adr`.
- Stakeholder input? Capture it as CTX via `/docops:new-ctx`.

After creating, remind the user that the default status is `backlog` — `docops status TP-xxx active` starts the work.
