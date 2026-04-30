---
name: close
description: Finish a task — flip status to done, refresh DocOps state, and stage a commit. Use when the user says "done with TP-X" / "mark TP-X complete".
---

# /docops:close

Close out a finished task.

Ask for the task ID if not provided. Then:

1. Verify the task's acceptance criteria are actually met. Read the
   task body, check for remaining TODOs, and (if code changed)
   eyeball the diff. If something is incomplete, stop and tell the user.

2. Flip the task's `status:` frontmatter to `done` (literal value;
   the enum is `backlog | active | blocked | done`) via `Edit` on the
   source file — do not hand-edit the index.

3. Run:

```
docops refresh
```

4. Stage every file relevant to the task (source changes + task
   frontmatter + `docs/.index.json` + `docs/STATE.md`), then propose a
   commit message of the form:

   ```
   <TP-ID>: <one-line summary of what shipped>

   <one or two sentences on what actually changed and why, pulled from
   the task's Goal + Acceptance>
   ```

   Confirm with the user before committing. Never commit without their
   explicit OK.

5. After the commit, run `/docops:progress` to surface the next move.

Do not mark a task `done` just because citing files changed — the
acceptance list is the contract.
