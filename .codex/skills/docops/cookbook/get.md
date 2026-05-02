---
description: Look up a single DocOps doc by ID (ADR-NNNN, CTX-NNN, TP-NNN) and print its indexed record. Use when you need the frontmatter and edges for one doc without reading the full file.
---

# Cookbook: get

## Context
Fetch one indexed doc. Run before reading the full file when you only
need title, status, and edges. `--json` returns the full `IndexedDoc`
for scripting or exact edge inspection.

## Input
A doc ID (`ADR-NNNN`, `CTX-NNN`, `TP-NNN`).

## Steps
1. Run:

   ```
   docops get <ID>
   docops get <ID> --json
   ```

## Confirm
Title, status, coverage/priority, dates, forward and reverse edges. Use
this to orient before reading the full file.
