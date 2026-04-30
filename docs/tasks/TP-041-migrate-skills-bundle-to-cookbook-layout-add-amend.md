---
title: Migrate skills bundle to cookbook layout + add amend/supersede/revise
status: done
priority: p2
assignee: nix
requires: [ADR-0031, ADR-0025]
depends_on: []
---

## Goal

Land the structural shift declared in ADR-0031 and ship the three
amendment-family cookbooks (`amend`, `supersede`, `revise`) called for
by ADR-0025 + ADR-0029 §Skills tier.

## Acceptance

1. `templates/skills/docops/<verb>.md` (18 files) move to
   `templates/skills/docops/cookbook/<verb>.md`. No content changes
   in the move commit.
2. `templates/skills/docops/SKILL.md` is rewritten:
   - Cookbook table links updated to `cookbook/<verb>.md`.
   - Three new entries — `amend`, `supersede`, `revise` — added with
     a short "when to use" summary inline so the rubric is visible
     without opening the chapter.
   - Variables section added (doc-dir tokens) per ADR-0031.
3. Three new cookbooks at `templates/skills/docops/cookbook/`:
   `amend.md`, `supersede.md`, `revise.md`. Each follows the
   Context / Input / Steps / Confirm convention and cites `docops amend`
   for the amend case + ADR-0025 inline.
4. `templates/CLAUDE.md.tmpl` and `templates/AGENTS.md.tmpl` get a
   short (≤10 line) Amendment-vs-Supersede-vs-Revise rubric block in
   the docops-managed section so the loaded harness instructions
   tell the LLM which lane to pick before any cookbook is read.
5. `templates.Skills()` walks `cookbook/`; output keys stay basename
   (`audit.md`, etc.).
6. `internal/upgrader/harness_codex.go` `FilenameFor` returns
   `docops/cookbook/<verb>.md`. Codex bundle delivery preserves the
   cookbook subdirectory.
7. Tests updated:
   - `templates/skills_lint_test.go` walks `cookbook/`.
   - `internal/upgrader/upgrader_test.go` asserts new cookbook path
     for the Codex bundle.
   - `internal/scaffold/scaffold_test.go` updated if it asserts flat
     layout.
8. Slash-command harnesses (`claude`, `cursor`, `opencode`) are
   unchanged — only the slash-deliverable subset still ships flat
   to `.claude/commands/docops/`.

## Non-goals

- Rewriting existing 18 cookbooks to the Context/Input/Steps/Confirm
  convention — that's a follow-up TP. The migration just moves files;
  shape normalisation comes later.
- Adding a `library.yaml`-style catalog (out of scope per ADR-0031
  Consequences).

## Notes

Order matters in the diff: move existing 18 files first (no content
change), then update SKILL.md + add new cookbooks + update Go code +
update tests. Keep the move commit reviewable.
