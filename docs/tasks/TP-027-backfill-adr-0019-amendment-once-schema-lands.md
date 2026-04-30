---
title: Backfill ADR-0019 tap-rename amendment once the schema lands
status: done
priority: p2
assignee: claude
requires: [ADR-0025, TP-026]
depends_on: []
---

## Goal

Once TP-026 ships the `amendments:` schema and `docops amend` CLI,
convert ADR-0019's HTML-comment amendment stub into a proper
frontmatter entry using the new machinery. ADR-0019 is the live
motivating case for ADR-0025; closing the loop proves the feature
end-to-end on a real ADR in the repo that authored the design.

## Context

On 2026-04-23 the tap/bucket repos were renamed from per-tool
(`logicwind/homebrew-docops`, `logicwind/scoop-docops`) to the
org-wide convention (`logicwind/homebrew-tap`,
`logicwind/scoop-bucket`) — pre-launch, zero external blast radius.
The migration is tracked in TP-024.

ADR-0019's "out of scope for v0.1.0" section originally named the
old repos. Because ADR-0025 (amendments as first-class) isn't live
yet, the correction was captured as:

- A `<!-- AMENDMENTS ... -->` HTML comment near the top of the ADR,
- An inline `[AMENDED 2026-04-23 editorial]` marker at the affected
  sentence,
- A human-readable `## Amendments` section at the end of the ADR.

Once TP-026 lands the `amendments:` YAML frontmatter schema + the
`docops amend` CLI, this task promotes that amendment to structured
metadata.

## Acceptance

- Invoke `docops amend ADR-0019` non-interactively, roughly:

  ```sh
  docops amend ADR-0019 \
    --kind editorial \
    --date 2026-04-23 \
    --by nix \
    --summary "Tap/bucket naming: per-tool → org-wide (pre-launch)" \
    --section "v0.1.0 scope" \
    --body-file - <<'EOF'
  See ADR-0019 §Amendments — migrated pre-launch to
  logicwind/homebrew-tap + logicwind/scoop-bucket. Deferral
  decision unchanged. TP-024 tracks the mechanical migration.
  EOF
  ```

- ADR-0019 frontmatter now contains an `amendments:` entry with:
  - `date: 2026-04-23`
  - `kind: editorial`
  - `by: nix`
  - `summary: "Tap/bucket naming: per-tool → org-wide (pre-launch)"`
  - `affects_sections: ["v0.1.0 scope"]`

- The HTML-comment stub is removed.
- The inline `[AMENDED 2026-04-23 editorial]` marker remains and
  validates against the new frontmatter entry.
- The `## Amendments` body section survives — the frontmatter is
  machine-readable metadata; the section is the human-readable
  narrative. Both are valuable.
- `docops validate` passes.
- `docops audit` reports no drift on ADR-0019.
- `docs/.index.json` surfaces the amendment; `docs/STATE.md`
  lists it in "Recent amendments" (if within the activity window).

## Notes

- Blocked on TP-026 landing — do not open a PR for this task until
  the schema and CLI are merged.
- If the `docops amend` CLI surface drifts during TP-026 from the
  shape specified in ADR-0025 §Decision/CLI, this task is the canary
  that catches it. File follow-up issues on TP-026's PR if so.
- Once this task is done, TP-024's "record the rename" note can
  refer to the structured amendment rather than the HTML comment.
