---
description: Finish a task — verify acceptance, flip status to done, refresh DocOps state, and propose a commit. Use when the user says "close TP-NNN", "done with TP-NNN", or "mark TP-NNN complete".
---

# Cookbook: close

## Context
Close a finished task: verify acceptance, flip `task_status: done`, refresh,
propose a commit. Never silently flip status; never `git commit` without
explicit OK.

## Input
Task ID (`TP-NNN`). Ask once if missing.

## Steps
1. Verify acceptance:

   ```
   docops get <TP-ID>
   ```

   Compare `## Goal` / `## Acceptance` against the diff and remaining TODOs.
   If incomplete, **stop** and surface what's missing.

2. Flip frontmatter `task_status: done` via `Edit` on the task file. Enum is
   literal — `backlog | active | blocked | done`. Do not edit
   `docs/.index.json` or `docs/STATE.md`.

3. Refresh:

   ```
   docops refresh
   ```

   Must report `OK`.

4. Stage source changes + the task file + `docs/.index.json` + `docs/STATE.md`,
   then propose:

   ```
   <TP-ID>: <one-line of what shipped>

   <one or two sentences from Goal + Acceptance>

   Closes <TP-ID>.
   ```

   Wait for explicit OK before committing.

5. Suggest `/docops:progress` (or hand off to `cookbook/next.md`).

## Confirm
The task ID with `task_status: done`, that `docops refresh` returned OK,
the commit hash if landed (or the staged file list awaiting confirmation),
and any acceptance gaps that were deferred or overridden.
