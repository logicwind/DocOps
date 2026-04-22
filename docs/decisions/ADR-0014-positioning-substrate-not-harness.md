---
title: Positioning — substrate for LLM-first dev, not harness or personas
status: accepted
coverage: not-needed
date: 2026-04-22
supersedes: []
related: [ADR-0011, ADR-0013, CTX-004]
tags: [positioning, scope, philosophy]
---

# Positioning — substrate for LLM-first dev, not harness or personas

## Context

The LLM-development tooling space in 2026 is crowded. Adjacent tools (GSD, GStack, BMAD, Spec Kit, Kiro, Agent OS, Backlog.md, Markplane) cover overlapping territory with different philosophies. DocOps risks diffusion if it tries to do everything.

## Decision

DocOps commits to a narrow, specific role:

**DocOps is the typed project-state substrate.** It provides context, structure, traceability, and gap detection. It does not orchestrate, prescribe, or execute.

Explicit non-goals for phase 1 (and likely forever):

- **No phase/wave orchestration** (GSD's domain).
- **No persona/role system** (GStack's domain).
- **No spec-as-contract code generation** (Kiro / Spec Kit / BMAD's domain).
- **No web UI** (Backlog.md / Linear's domain).
- **No execution harness** (Claude Code plan mode already covers this).
- **No automated PR creation, code generation, or implementation**.
- **No attempt to replace or subsume GSD.** DocOps is lighter; GSD is heavier. Users choose based on their taste for process.

Explicit design partners:

- **Native LLM plan-mode** (Claude Code / Cursor / Aider) — DocOps feeds them context; they plan and execute.
- **GStack-style role skills** — DocOps provides the typed state that roles read to form their perspective.
- **AGENTS.md** — DocOps emits one that points at its commands and schemas.

## Rationale

- A tool that does one thing well is easier to adopt, trust, and maintain.
- The alignment-graph + coverage-audit wedge is genuinely unclaimed in the landscape (see CTX-003).
- Trying to compete with GSD on orchestration would mean copying its complexity without its maturity.
- Users who want orchestration can pair DocOps with GSD or similar; the data models can coexist because DocOps is file-first.

## Consequences

- Feature requests to add workflow orchestration must be refused (or routed to a separate companion project).
- Documentation must consistently position DocOps as "the data layer for agent-native PM," not as a full PM framework.
- If the substrate/harness separation proves unworkable in practice, this ADR is the thing to supersede.
- Branding and marketing copy must not imply DocOps "manages your project." It "describes and validates it."
