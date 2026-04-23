---
name: graph
description: Walk the typed edge graph outward from one doc to its neighbours (requires, related, supersedes, reverse edges). Use to understand "what depends on what" before touching an ADR or task.
---

# /docops:graph

Print the typed edge graph starting at one doc.

```
docops graph ADR-0010
docops graph ADR-0010 --depth 2
docops graph TP-029 --json
```

`--depth N` (default 1) controls traversal depth. Depth 1 shows direct
neighbours; higher depths show transitive closure. Edges include both
source fields (`requires`, `related`, `supersedes`, `depends_on`) and
computed reverse edges (`required_by`, etc. — see ADR-0006).

Use this before superseding an ADR or removing a task: the graph shows
every doc that would break.
