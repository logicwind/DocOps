---
name: get
description: Look up a single DocOps doc by ID (ADR-nnnn, CTX-nnn, TP-nnn) and print its indexed record. Use when you need the frontmatter and edges for one doc without reading the full file.
---

# /docops:get

Fetch one indexed doc by ID.

```
docops get ADR-0010
docops get TP-029 --json
```

The plain output shows title, status, coverage/priority, dates, and both
forward and reverse edges (computed). `--json` returns the full
`IndexedDoc` — use it when scripting or when you need the exact edge
shape.

When a user pastes an ID into chat, run `docops get <ID>` first to
orient before reading the full file.
