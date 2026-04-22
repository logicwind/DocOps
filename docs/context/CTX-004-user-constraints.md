---
title: Author constraints and philosophical guardrails
type: memo
supersedes: []
---

# Author constraints and philosophical guardrails

Explicit rules the author (Nachiket) placed on the design. These govern what DocOps will and will not do. Future agents should treat violations of these as red flags.

## Hard constraints

1. **Do not dictate workflow.** DocOps gives agents context + structure + traceability + gap detection. It does not prescribe phases, waves, discussion gates, execution patterns, or PR workflows. The LLM's native plan and execute capabilities do the actual work.

2. **Pair with, do not replace, existing agent tooling.** Notably: GStack-style role skills. DocOps provides the typed state those tools read; they provide the perspective that reads it.

3. **Language-agnostic distribution.** No assumption that the user has Node, Bun, Python, Go, or any specific runtime. DocOps ships as a standalone binary (downloadable directly, via Homebrew/Scoop, etc.) with optional convenience shims for package managers.

4. **CLI before MCP.** Phase 1 exposes capabilities as a CLI with `--json` output on every command. MCP support is a phase 2+ consideration, not a foundation. The CLI must work from any agent, any shell, any CI.

5. **Bare-minimum frontmatter.** Add a field only when both:
   - It is nearly certain to be used in the first two weeks of real use.
   - Its absence would force information into body prose where the LLM cannot structurally query it.
   Examples of fields added under this rule: task `priority`, task `assignee`, ADR `coverage`, CTX `supersedes`.

6. **No web UI in phase 1.** DocOps is CLI + files + agent skills. If a UI is ever built, it is downstream of everything else and optional.

7. **No code generation, no execution harness, no auto-PR.** DocOps never writes code for the user. It only writes/updates structured documentation.

## Soft constraints

- Prefer boring over clever. The point of DocOps is to be reliable infrastructure.
- Commit messages and git history should remain meaningful. Avoid field designs that generate churn (e.g., frequently-toggled status fields living in source frontmatter when they could be computed).
- If a feature can be moved from source (human-authored) frontmatter to the computed index, prefer the computed side — fewer fields to maintain, less drift.

## Red flags for future design decisions

If a proposed change matches any of these, stop and reassess:

- Adds a prescribed workflow step the user must follow.
- Introduces a runtime dependency specific to a single language ecosystem.
- Requires a background daemon or server to be run by default.
- Adds frontmatter fields to satisfy a "maybe someday" use case.
- Overlaps meaningfully with GSD's phase orchestration.
- Overlaps meaningfully with GStack's role/persona model.
- Requires the user to configure hosts (Claude Desktop, IDE settings) before the tool works.

## North-star test

An LLM agent opens a DocOps-enabled repo cold. Reading only `AGENTS.md`, `docs/STATE.md`, and the few files those point it at, the agent can pick the next task, understand the decisions constraining it, produce a change, and verify no gaps opened — all without asking the human for clarification on project state. If this works, DocOps is done. If this does not work, DocOps is the problem.
