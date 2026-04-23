---
name: refresh
description: Run validate + index + state in one pass after any doc edit. Replaces the three-command chain. Use this after every document change.
---

# /docops:refresh

Run after every document edit to keep the project state consistent.

```
docops refresh
```

Collapses the three-command post-edit chain into one:

1. **validate** — schema + graph invariants (stops here on failure).
2. **index** — rebuilds `docs/.index.json`.
3. **state** — regenerates `docs/STATE.md`.

If validate fails, index and state are skipped. Fix the validation errors first, then re-run.

For CI or structured output:

```
docops refresh --json
```

The `--json` flag emits `{"ok": true, "steps": [...]}` with per-step details. Exit code is 0 when all steps pass, 1 when validate fails, 2 on bootstrap error.
