---
title: Landscape research — LLM-first PM tooling, April 2026
type: research
supersedes: []
---

# Landscape research — LLM-first PM tooling (April 2026)

Captured from web research during design. Keeps the "why we are not rebuilding X" reasoning available to future agents.

## Closest prior art

**Backlog.md** (`MrLesk/Backlog.md`) — markdown + YAML + CLI + web UI + MCP. Tasks/docs/decisions stored as peers, no alignment graph. Active; strong distribution (brew/npm/bun). The closest direct competitor.

**Markplane** (`zerowand01/markplane`) — Go binary, built-in MCP, auto-generates a compressed `.context/` directory of summaries. Spiritually a sibling of our `.index.json`. Active 2025–26.

**Markdown Projects** (`markdownprojects.com`) — each issue as a markdown file with YAML frontmatter in `.mdp/`. Thinner than the above.

## ADR-specific tools (older, stable)

**adr-tools** (Nat Pryce), **MADR**, **log4brains**, **pyadr** — all handle decisions only. None integrate tasks or stakeholder context. None have a computed index. None expose MCP. MADR is the de-facto frontmatter convention to inherit from.

## Adjacent frameworks

**GSD — Get Shit Done** (`gsd-build/get-shit-done`, site `gsd.build`). Phase-based harness with `.planning/PROJECT.md`, `REQUIREMENTS.md`, `ROADMAP.md`, `STATE.md`, decision IDs, workstreams, milestone audits, and parallel waves. Built on the Pi-SDK for direct Claude Code harness control. Actively maintained. Its footprint overlaps with DocOps but its philosophy differs — GSD orchestrates process; DocOps provides substrate. Rejected as a compatibility target: "crazy slow, difficult to keep on track" in real use.

**GStack** (`garrytan/gstack`) — 23 opinionated role/skill files (CEO, Designer, Eng Manager, QA, etc.). Constrains *decision-making perspective*, not process. DocOps pairs well with GStack: DocOps provides the typed state GStack roles read.

**BMAD Method** — personas-as-code (PM / Architect / Dev / SM / UX) + PRD → user stories. Enterprise framing; heavier than DocOps intends.

**Agent OS** (`buildermethods/agent-os`) — standards + specs + workflows as reusable markdown blocks, injection into host agents.

**GitHub Spec Kit** — spec-as-contract, greenfield.

**OpenSpec** (`Fission-AI/OpenSpec`) — brownfield delta markers (ADDED / MODIFIED / REMOVED). Interesting for task state transitions later.

**AWS Kiro** — spec-as-source-of-truth IDE; GA Nov 2025. Not file-format-open the way AGENTS.md is.

## Standards converging in 2026

1. **AGENTS.md** — the "agent instructions" file at repo root. Donated to Linux Foundation / Agentic AI Foundation in Dec 2025. Adopted by Copilot, Codex, Cursor, Jules/Gemini, Factory, Amp, Windsurf, Zed, RooCode. 20K+ repos. DocOps must emit/respect it.
2. **MCP** — default agent-integration interface across Linear, Backlog.md, Markplane, GitHub. Worth adopting eventually; not phase 1.
3. **Markdown + YAML frontmatter** — the universal substrate.
4. **A computed/compressed index** for agent consumption — Markplane `.context/`, Aider repo map, Agent OS index, GSD `.planning/`, Kiro spec graph. Everyone builds one; DocOps fits in.

## What is unclaimed (DocOps' wedge)

1. **Enforced alignment graph** — hard rule that every task must cite ≥1 ADR or CTX. Nobody does this.
2. **Typed edges beyond supersedes** — `supersedes / related / requires` as a three-edge graph with computed reverse edges, auto-generated on every index build. Nobody standardizes this.
3. **CTX as a first-class doc type.** Most tools blur requirements into tasks or omit them. Stakeholder input as schema-typed doc is open.
4. **Coverage audit as primary workflow.** Structural gap detection (graph holes) and semantic coverage review (LLM-judged) as a core surface, not a side utility.

## Steal vs. differentiate

**Steal from:**
- MADR — ADR frontmatter conventions.
- pyadr — explicit lifecycle semantics (supersedes chains).
- Backlog.md — CLI + brew/npm distribution story.
- Markplane — compressed `.index.json` concept.
- AGENTS.md — adopt, don't reinvent.
- GSD — directory layout cues (`.planning/`-style dotdirs) only where it helps, no deeper compatibility.
- Linear MCP — tool naming conventions if/when DocOps ships MCP in phase 2.
- OpenSpec — ADDED / MODIFIED / REMOVED markers as a future task-state-transition idea.

**Differentiate hard on:**
- Hard citation rule (tasks refuse to save without `requires`).
- Three-edge typed graph + computed reverse edges.
- CTX as first-class.
- Coverage audit (structural + semantic) as a primary CLI command.

## Sources

- https://github.com/npryce/adr-tools
- https://adr.github.io/madr/
- https://github.com/MrLesk/Backlog.md
- https://github.com/zerowand01/markplane
- https://agents.md/
- https://github.com/bmad-code-org/BMAD-METHOD
- https://github.com/buildermethods/agent-os
- https://github.com/github/spec-kit
- https://github.com/Fission-AI/OpenSpec
- https://kiro.dev/
- https://github.com/gsd-build/get-shit-done
- https://github.com/garrytan/gstack
- https://linear.app/changelog/2026-02-05-linear-mcp-for-product-management
