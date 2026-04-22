---
title: skills.sh compatibility — .agents/skills/ as canonical skill store with symlinks
status: accepted
coverage: required
date: 2026-04-23
supersedes: []
related: [ADR-0021]
tags: [cli, skills, init, upgrade, interop]
---

# skills.sh compatibility — .agents/skills/ as canonical skill store with symlinks

## Context

DocOps currently writes skills into two tool-specific directories during
`docops init` and `docops upgrade`:

- `.claude/skills/docops/` — consumed by Claude Code
- `.cursor/commands/docops/` — consumed by Cursor

This layout was chosen pragmatically based on what each tool reads, but it
has two problems:

1. **Wrong Cursor path.** Cursor's skills.sh entry uses `.agents/skills/`,
   not `.cursor/commands/`. The `.cursor/commands/` convention predates the
   skills.sh ecosystem and is not where Cursor looks for installable skills.

2. **Not interoperable.** The [skills.sh](https://skills.sh) ecosystem has
   emerged as the de-facto cross-tool skill registry. Its CLI (`npx skills
   add`) stores skills in one canonical location (`.agents/skills/`) and
   creates **symlinks** into tool-specific dirs for the tools that need them
   (Claude Code, Windsurf, etc.). Tools that natively read `.agents/skills/`
   (Cursor, Cline, GitHub Copilot, Gemini CLI, and a dozen others) receive no
   symlink — they consume canonical directly.

   A project initialized by DocOps today is invisible to `npx skills list`,
   cannot co-exist cleanly with skills installed via `npx skills add`, and
   forces users who adopt skills.sh after DocOps init to reconcile two
   parallel layouts.

## Decision

Change `docops init` and `docops upgrade` to adopt the skills.sh layout:

**Canonical store (always written):**
```
.agents/skills/docops/    ← real files, committed to the repo
```

**Per-tool symlinks (created for tools that have their own skillsDir):**
```
.claude/skills/docops     → symlink → ../../.agents/skills/docops
.windsurf/skills/docops   → symlink → ../../.agents/skills/docops
```

Tools that read `.agents/skills/` directly (Cursor, Cline, GitHub Copilot,
Gemini CLI, Kilo, Roo, OpenAI Codex, and most others) need no symlink.

**Agent detection during `docops init`:**

`docops init` will detect which agents are present on the machine by checking
for the existence of known config directories (e.g. `~/.claude`, `~/.cursor`,
`~/.windsurf`). The detected set is presented to the user for confirmation
before any symlinks are created. Unknown or future agents whose `skillsDir`
is `.agents/skills/` benefit automatically — no symlink needed.

**Flag `--agents`:**

Add `--agents <list>` to `docops init` and `docops upgrade` to override
auto-detection. Accepts a comma-separated list of agent slugs matching the
skills.sh agent registry (e.g. `--agents claude-code,windsurf`). Useful in
CI / headless environments.

**Symlink commit policy:**

Symlinks should be committed to the repository so that teammates who clone
the project get the correct layout without running `docops init`. The
`.gitignore` template must not exclude `.agents/` or `.claude/skills/`.

**`docops upgrade` — migrating v0.1.x projects:**

On an existing project that has `.claude/skills/docops/` as real files (no
`.agents/skills/` present), `upgrade` performs a one-time migration:
1. Copy `.claude/skills/docops/` → `.agents/skills/docops/`.
2. Replace `.claude/skills/docops/` with a symlink.
3. Delete `.cursor/commands/docops/` if present (wrong path; Cursor reads
   `.agents/skills/` directly — no symlink needed).
4. Report each action with the same `+/~/- /=` sigils as the rest of upgrade.

The migration is idempotent: if `.agents/skills/docops/` already exists and
`.claude/skills/docops` is already a symlink, upgrade is a no-op for that
path.

## Rationale

- **One canonical copy, zero drift.** All agents read the same bytes —
  either directly from `.agents/skills/docops/` or via a symlink that points
  there. There is no risk of the Claude-side and Cursor-side diverging.

- **skills.sh interop is additive.** A project initialized by DocOps can
  coexist with skills installed via `npx skills add` because both share
  `.agents/skills/` as the canonical root. `npx skills list` will see
  DocOps-managed skills alongside any others.

- **Auto-detection is low-friction, explicit override is available.** Most
  developers run one or two agents. Detection based on `~/.<agent>` dirs
  covers the common case; `--agents` handles CI and multi-machine edge cases.

- **Correct Cursor path.** Removing `.cursor/commands/docops/` and relying
  on `.agents/skills/` fixes a bug — Cursor never read from
  `.cursor/commands/` in the skills.sh model.

## Consequences

- `docops init` grows agent-detection logic and a new `--agents` flag. The
  TTY confirmation flow from ADR-0020 covers the prompt; the new step is
  an extra section "symlinks to create" in the confirmation summary.

- `docops upgrade` (TP-018) must be updated before shipping: the target
  paths change from `.claude/skills/docops/` + `.cursor/commands/docops/`
  to `.agents/skills/docops/` + conditional symlinks. TP-018 depends on
  this ADR and should be updated accordingly.

- `.agents/` is a new top-level directory in initialized repos. The
  `.gitignore` template and the meta/product boundary note in `AGENTS.md`
  must acknowledge it.

- The shipped-bundle manifest (`.docops-manifest` from ADR-0021 / TP-018)
  lives inside `.agents/skills/docops/`, not in the symlink targets.

- Windows junction support: `createSymlink` in skills.sh uses `junction`
  type on Win32. DocOps's Go implementation should do the same
  (`os.Symlink` works on Windows 10+ with Developer Mode; fall back to
  copying with a warning if symlink creation fails).
