---
description: Walk the typed edge graph outward from one doc to its neighbours (requires, related, supersedes, reverse edges). Use to understand "what depends on what" before touching a CTX, ADR, or task.
---

# Cookbook: graph

## Context
Print the typed edge graph starting at one doc. `--depth N` (default 1)
controls traversal depth. `--json` emits structured output; each node
carries a `referenced_by: [{id, edge}]` array (computed reverse edges).

When NOT to use:
- **Cross-cutting rereads** (many docs at once): `docops audit` +
  `docops list --kind ADR --stale` cover more ground.
- **Freshness checks**: `docops list --kind ADR --since YYYY-MM-DD`.
- **Discovery by keyword**: start with `cookbook/search.md`, then pivot
  here once you have IDs.

## Input
A doc ID. Optional `--depth N`, `--json`.

## Steps
1. Run:

   ```
   docops graph <ID>
   docops graph --depth 2 <ID>
   docops graph --json <ID>
   ```

2. (Optional) use one of the impact-map cheatsheet patterns below.

## Cheatsheet — 7 impact-map patterns

### 1. "I'm about to edit a CTX — what else needs review?"

```
docops graph --json <CTX-ID> \
  | jq -r '.nodes[] | select(.id == "<CTX-ID>") | .referenced_by[]?.id'
```

Every ADR / TP that cites the CTX in `requires:` or `related:`. Read
those before editing the CTX body.

### 2. "What does this ADR depend on, and what depends on it?"

```
docops graph <ADR-ID>
```

Outgoing + incoming edges. Run before superseding — incoming
tasks/ADRs may need amendments, not silent rewrites.

### 3. "Which open TPs would break if I revert this ADR?"

```
docops graph --json <ADR-ID> \
  | jq -r '.nodes[] | select(.id == "<ADR-ID>") | .referenced_by[]? | select(.id | startswith("TP-")) | .id' \
  | while read id; do
      s=$(docops get --json "$id" | jq -r .task_status)
      [ "$s" != "done" ] && echo "$id ($s)"
    done
```

Reverse edges filtered to unfinished TPs.

### 4. "Trace why this task exists, all the way back to the PRD"

```
docops graph --depth 3 <TP-ID>
```

Three hops: requires → cited CTXs → related context. Read the chain
before coding.

### 5. "Am I about to touch a superseded decision?"

```
docops get --json <ADR-ID> | jq '{supersedes, superseded_by, status}'
```

If `superseded_by` is non-null or `status == "superseded"`, edit the
newer ADR instead.

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

Zero reverse edges → either a missing ADR or stale context. Pair with
`cookbook/plan.md`.

### 7. "Body-text mentions graph misses"

```
docops search "<TARGET-ID>"
```

Graph walks frontmatter citations only. Search catches prose mentions
that aren't yet typed edges — promote those to proper `requires:` /
`related:` entries.

## Confirm
For a basic walk: node + edge counts and the immediate neighbour IDs.
For a cheatsheet pattern: the IDs that match the question (or "none"
when the impact set is empty).
