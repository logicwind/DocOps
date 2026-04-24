---
name: graph
description: Walk the typed edge graph outward from one doc to its neighbours (requires, related, supersedes, reverse edges). Use to understand "what depends on what" before touching a CTX, ADR, or task.
---

# /docops:graph

Print the typed edge graph starting at one doc.

```
docops graph ADR-0010
docops graph --depth 2 ADR-0010
docops graph --json TP-029
```

Flags precede the positional ID: `--depth N` (default 1) controls
traversal depth; `--json` emits structured output. Depth 1 shows direct
neighbours; higher depths show transitive closure. Each node in the
JSON output carries a `referenced_by: [{id, edge}]` array — the
computed reverse edges (per ADR-0006).

## Cheatsheet — 7 impact-map patterns

These are the most common "before I touch X, what else has to move?"
flows. Each is one command; `--json` + `jq` gives an agent-friendly
ID list when you only need names.

### 1. "I'm about to edit CTX-X — what else needs review?"

```
docops graph --json CTX-002 \
  | jq -r '.nodes[] | select(.id == "CTX-002") | .referenced_by[]?.id'
```

Prints every ADR / TP that cites CTX-002 in its `requires:` or
`related:`. Read those before changing the CTX body — a CTX edit can
silently invalidate downstream decisions.

### 2. "What does this ADR actually depend on, and what depends on it?"

```
docops graph ADR-0028
```

Human-readable view: outgoing edges (what this ADR cites) and incoming
(what cites it). Run before superseding: incoming tasks/ADRs may need
amendments (see ADR-0025) rather than silent rewrites.

### 3. "Which open TPs would break if I revert this ADR?"

```
docops graph --json ADR-0013 \
  | jq -r '.nodes[] | select(.id == "ADR-0013") | .referenced_by[]? | select(.id | startswith("TP-")) | .id' \
  | while read id; do
      s=$(docops get --json "$id" | jq -r .task_status)
      [ "$s" != "done" ] && echo "$id ($s)"
    done
```

Filters the reverse-edge set to unfinished TPs. Anything printed is a
task whose scope depends on the ADR you're about to change.

### 4. "Trace why this task exists, all the way back to the PRD"

```
docops graph --depth 3 TP-033
```

Walks outward three hops: direct requires (usually ADRs), then the
CTXs those ADRs cite, then anything those CTXs relate to. Reading the
whole chain before coding prevents re-litigating decisions already
documented upstream.

### 5. "Am I about to touch a superseded decision?"

```
docops get --json ADR-0013 | jq '{supersedes, superseded_by, status}'
```

If `superseded_by` is non-null or `status == "superseded"`, edits
should land in the newer ADR — not here. The validator catches
dangling `supersedes:` edges but not the semantic "should I be editing
this at all".

### 6. "List CTX docs nothing cites (orphans)"

```
docops list --kind CTX --json \
  | jq -r '.[].id' \
  | while read id; do
      refs=$(docops graph --json "$id" \
        | jq --arg id "$id" '[.nodes[] | select(.id == $id) | .referenced_by[]?] | length')
      [ "$refs" = "0" ] && echo "$id (orphan)"
    done
```

A CTX with zero reverse edges isn't yet "decided on" — either an ADR
is missing or the context has gone stale. Pair with `/docops:plan` to
close the gap.

### 7. "Body-text mentions graph misses"

```
docops search "CTX-002"
```

Graph walks *frontmatter citations only*. If another doc mentions the
target in prose but doesn't list it in `requires:` / `related:`, graph
won't see it. Run search as a safety net after the graph walk — any
hit that isn't already in the graph output is an un-typed reference
worth promoting to a proper edge.

## When NOT to use graph

- **Cross-cutting rereads**: if you're restructuring many docs at once,
  `docops audit` + `docops list --kind ADR --stale` cover more ground
  than per-doc graph walks.
- **Freshness checks**: `docops list --kind ADR --since 2026-04-01`
  is the right tool for "what changed recently", not graph.
- **Discovery by keyword**: start with `/docops:search`, then pivot
  into graph once you know the IDs.
