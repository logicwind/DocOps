---
title: Ship /docops:onboard skill — scan + interview + draft CTX/ADRs
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0033]
depends_on: []
---

## Goal

Add an `onboard` cookbook chapter under the `docops` skill bundle so
agents can drive a guided onboarding flow on any repo.

## Scope

1. **New skill chapter** `templates/skills/docops/cookbook/onboard.md`
   following the umbrella + cookbook layout (ADR-0031). Sections:
   *Context / Input / Steps / Confirm*. Steps spell out:
   - Inspect: README, every package manifest present,
     `git log --oneline -20`, top-level dirs only (no recursive scan).
   - Summarize: print an inferred summary of stack, scale, and
     primary purpose.
   - Interview: at most 3–5 clarifying questions covering users,
     pain, 3-month goal, hard constraints. Skip if the user invoked
     with `--auto` or said "skip the interview".
   - Draft: 1× CTX-001 (vision + goals) and 1–3 ADRs for load-bearing
     decisions visible in the code (framework, auth, deploy target,
     datastore — only what code-evidence supports).
   - Confirm: show drafts inline, ask ship / iterate / abort.
   - Write: call `docops new ctx` and `docops new adr` with
     `--body -` heredoc per CLAUDE.md guidance.
   - Handoff: end with the standard next-step block (see TP for §3
     of ADR-0033 — generic affordance landing in CLI).
2. **Register** the chapter in `templates/skills/docops/SKILL.md`
   index alongside the other cookbook entries.
3. **Mirror via `docops upgrade`** to all harness destinations
   (`.claude`, `.cursor`, `.codex`, `.opencode`). Verify the manifest
   files pick it up.
4. **Cookbook smoke test**: run the skill end-to-end in a throwaway
   brownfield fixture and confirm CTX-001 + at least one ADR are
   produced and pass `docops validate`.

## Out of scope

- Backfilling ADRs from `git log` / inline comments. Code-evidence
  surface only.
- Importing existing ADR folders (adr-tools / MADR / log4brains).
- Auto-classifying or grading the drafts. The user reviews.

## Done when

- `cookbook/onboard.md` exists, follows the standard shape, and is
  linked from SKILL.md.
- `docops upgrade` in a test repo brings the chapter to
  `.claude/skills/docops/cookbook/onboard.md` and the parallel codex /
  cursor / opencode locations.
- A clean dry-run on a brownfield fixture produces a valid CTX +
  ADR(s) that pass `docops validate`.
