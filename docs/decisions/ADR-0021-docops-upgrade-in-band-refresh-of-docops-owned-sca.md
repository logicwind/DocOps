---
title: docops upgrade — in-band refresh of DocOps-owned scaffolding
status: accepted
coverage: required
date: 2026-04-22
supersedes: []
related: [ADR-0015, ADR-0016, ADR-0020]
tags: [cli, upgrade, lifecycle]
---

# docops upgrade — in-band refresh of DocOps-owned scaffolding

## Context

Between v0.1.0 and v0.1.1 the product added a new `/docops:refresh`
skill, removed an obsolete `/docops:next` skill, updated three other
skill files with heredoc body examples, and refreshed the
`AGENTS.md` template. A user who initialized their project on v0.1.0
has no in-band way to pull any of that after `brew upgrade docops`.
Their only option today is `docops init --force`, which is too blunt
— it also overwrites `docops.yaml` and the pre-commit hook, both of
which users legitimately customize.

This gap will recur every point release that touches templates. It
becomes more painful as template surface area grows (skills for
future commands, additional schema files, updated agent instructions).

## Decision

Ship `docops upgrade` as a new subcommand whose contract is narrower
than `docops init`:

- Target only **DocOps-owned, never-user-edited** scaffolding:
  - `.claude/skills/docops/*.md` and `.cursor/commands/docops/*.md`
    — replaced with the shipped set; skill files removed upstream
    are deleted locally too.
  - JSON Schemas under `docs/.docops/schema/*.schema.json` — rewritten
    from the shipped templates (same as `docops schema` does today).
  - The `<!-- docops:start -->` / `<!-- docops:end -->` block inside
    `AGENTS.md` — merged in place, preserving everything outside the
    markers (same mechanism TP-007 already ships).

- **Never touch** by default:
  - `docops.yaml` — user-owned config.
  - `.git/hooks/pre-commit` — user may have chained other hooks.
  - `docs/{context,decisions,tasks}/` — user content.
  - `docs/.index.json`, `docs/STATE.md` — computed; `docops refresh`
    handles them.
  - `docs/.docops/counters.json` — state.

- **Flags**:
  - `--config` — also re-write `docops.yaml`. Opt-in.
  - `--hook` — also re-install the pre-commit hook. Opt-in.
  - `--dry-run` — print the diff (what would change) without writing.
  - `--yes` / `-y` — skip the TTY confirmation, mirroring `init`.
  - No positional `[dir]`; `upgrade` always targets cwd (or the
    nearest ancestor containing a `docops.yaml`). Upgrading another
    project should be done from that project's directory — matches
    every other non-init command.

- **Removed-skill handling**: any file under
  `.claude/skills/docops/` or `.cursor/commands/docops/` that does not
  appear in the shipped skill bundle is deleted. This is the mechanism
  that makes `next.md` go away in a v0.1.0 → v0.1.1 upgrade. Files
  under those directories that the user placed there themselves are
  out of scope — DocOps owns the `docops/` subdirectory entirely.

- **Output**: human-readable diff-like summary per path, e.g.
  `~ .claude/skills/docops/init.md   (refreshed)`,
  `+ .claude/skills/docops/refresh.md (new)`,
  `- .claude/skills/docops/next.md    (removed)`,
  `~ AGENTS.md                        (block refreshed)`.
  `--json` returns the same shape as structured data.

## Rationale

- The friction is structural: every template-touching release creates
  this upgrade conversation. A dedicated command makes the path
  obvious and reviewable.
- Narrowing the blast radius from `init --force` means users stop
  treating upgrade as "risky" — they can run it after every version
  bump without losing config.
- The "DocOps owns the `docops/` subdirectory under `.claude/skills/`
  and `.cursor/commands/`" rule is clean — it mirrors how Homebrew
  owns `/opt/homebrew/Cellar/<formula>/` but not the rest of the
  machine. Users who want to add their own skills put them next to
  `docops/`, not inside it.
- Keeping `--config` and `--hook` opt-in avoids the footgun without
  removing power-user access. Power users who have scripted a
  specific `docops.yaml` shape can still refresh it explicitly.

## Consequences

- `docops init --force` stays as the nuclear option and keeps its
  existing semantics. `upgrade` does not supersede it; they serve
  different situations (greenfield init vs. in-place version bump).
- The version-bump runbook in the README and in the user-facing
  `AGENTS.md.tmpl` should point at `docops upgrade` instead of
  `docops init --force`.
- A file-delete operation inside `.claude/` and `.cursor/` is newly
  introduced. Scope limit: only within the `docops/` subdirectory
  under each; DocOps never touches sibling directories or files.
- The shipped-bundle manifest becomes a load-bearing abstraction —
  the binary must know at compile time which files belong to it,
  which is already true via `templates/templates.go` `//go:embed`.
  No new machinery required.
- When a future release changes `docops.yaml`'s default shape (new
  fields, renamed paths), users who do not pass `--config` will drift.
  That is acceptable — docops.yaml has a `version:` key; a future
  migration step can run on demand.
