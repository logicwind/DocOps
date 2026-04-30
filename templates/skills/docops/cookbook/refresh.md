---
name: refresh
description: Run validate + index + state in one pass after any doc edit. Replaces the three-command chain. Use this after every document change.
---

# Cookbook: refresh

## Context
Post-edit chain: validate → index → state. If validate fails, the rest
is skipped. Run after every document change.

## Input
None.

## Steps
1. Run:

   ```
   docops refresh
   ```

   For CI / scripted output:

   ```
   docops refresh --json
   ```

   `--json` emits `{"ok": true, "steps": [...]}`. Exit codes: 0 on full
   pass, 1 on validate failure, 2 on bootstrap error.

2. If validate fails, surface the errors and **stop**. Do not flip on
   to indexing — fix the source first, then re-run.

## Confirm
Per-step OK / failure counts. If anything failed, the failing step's
error verbatim.
