---
title: Skill bundle uses umbrella SKILL.md plus cookbook chapters
status: draft
coverage: required
date: "2026-04-30"
supersedes: []
related: [ADR-0029]
tags: []
---

## Context

ADR-0029 (command-surface tiering) declared three audiences — slash, skills,
CLI — and assigned skills to the LLM-NL-dispatch tier with the shape
"Many, granular." The implication taken at the time was: each capability
(`get`, `amend`, `close`, `audit`, …) ships as its **own** top-level
skill so the harness's skill-matching layer ranks the right one for any
NL intent.

In practice that produced 18 sibling skill files in `templates/skills/docops/`
plus an umbrella `SKILL.md` whose only role was an index table linking
to them. Every skill duplicated frontmatter, doc-tree path strings,
citation rules, and post-check mechanics. Drift was inevitable.

The disler/the-library project (https://github.com/disler/the-library)
solves the same shape problem with one router skill + a `cookbook/`
directory: `SKILL.md` is short and trigger-focused, the `cookbook/`
chapters carry the procedure detail with a fixed Context / Input / Steps
/ Confirm convention. The skill is a router; the cookbook is the runtime.

For docops specifically the umbrella shape is the right call:

- Most NL intents that mention `amend`, `close`, `audit`, etc. are
  unambiguously docops-shaped — harness-level granularity buys little.
- The convention surface (variables, citation rules, post-checks) lives
  once in `SKILL.md` instead of being copy-pasted into 18 files.
- Adding a new verb is one cookbook entry plus one table line, not a
  new top-level skill plus a new fixture in the dispatcher test set.

## Decision

The DocOps skill is **one umbrella skill** with **many cookbook chapters**:

```
templates/skills/docops/
  SKILL.md                    ← umbrella router (frontmatter, variables,
                                cookbook table)
  cookbook/<verb>.md          ← one chapter per granular capability
```

Cookbook chapters use a fixed section convention:

- `## Context` — what this capability does and when to invoke it.
- `## Input` — what the user supplies (or what the agent should infer).
- `## Steps` — numbered, executable; literal `docops` invocations in
  fenced code blocks.
- `## Confirm` — what the agent reports back when done.

Frontmatter on each cookbook stays minimal — `description:` only
(used as a trigger hint when the chapter is read on its own). The
umbrella `SKILL.md` is the main `name:` + harness-level
trigger-description surface.

Slash commands stay unchanged. The slash-deliverable subset
(`init`, `progress`, `next`, `do`, `plan` per ADR-0029) still ships
flat to `.claude/commands/docops/<verb>.md` — slash commands are
self-contained (no inter-file references) and don't gain anything
from the cookbook layout. Only the skill-bundle harness (Codex) and
source organisation use `cookbook/`.

## Rationale

- **One source of truth.** Variables (doc dirs, doc-kind taxonomy,
  invariants) live in `SKILL.md`. Cookbooks reference them as `<TOKEN>`.
  Updating a convention is one edit.
- **Trigger surface concentrates.** The harness loads one skill
  description; the agent then picks the cookbook by reading the
  router table. Fewer fuzzy-matched skill descriptions competing.
- **Pattern is well-trodden.** disler/the-library uses this exact shape
  for a meta-skill that manages other skills. Anthropic skill-creator
  guidance encourages progressive disclosure — short SKILL.md,
  detail-on-demand.
- **Slash commands unaffected.** The migration is invisible to users
  who only invoke `/docops:*` slashes. Codex bundle gains a `cookbook/`
  subdirectory but the entry point (SKILL.md) stays at the root.

## Consequences

- All current per-verb files move from `templates/skills/docops/<verb>.md`
  to `templates/skills/docops/cookbook/<verb>.md`. Content is preserved;
  only the path changes for this ADR. Cookbook normalisation against
  the Context / Input / Steps / Confirm convention is a follow-up TP.
- `templates.Skills()` walks the `cookbook/` subtree. Output keys
  remain basenames (e.g. `audit.md`) so callers don't shift.
- `harness_codex.FilenameFor` returns `docops/cookbook/<verb>.md`;
  Codex bundle layout becomes:
  ```
  .codex/skills/docops/
    SKILL.md
    cookbook/<verb>.md
    .docops-manifest
  ```
- The bundle manifest records cookbook-relative basenames so
  unknown-files detection still works.
- Slash-command harnesses (`claude`, `cursor`, `opencode`) are
  unchanged — they still write only the slash-deliverable subset
  flat to `.claude/commands/docops/`.
- ADR-0029's Skills tier description shifts from
  "Many, granular skills" to "One umbrella + many cookbook chapters."
  The rest of ADR-0029 (slash narrowing, CLI completeness) stands.

## Out of scope

- Normalising every cookbook to the Context/Input/Steps/Confirm
  shape. Many existing skills don't follow it; rewriting all 18 in
  one pass would balloon the diff. A follow-up TP will retrofit
  multi-step skills (`do`, `plan`, `close`); trivial verbs
  (`get`, `list`, `state`) can stay in their current shape.
- A separate `library.yaml`-style catalog or distribution mechanism.
  DocOps ships its own skills via `docops upgrade`; we are not a
  registry.
