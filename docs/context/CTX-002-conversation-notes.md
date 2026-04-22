---
title: Idea evolution — from ADR-only observation to DocOps
type: notes
supersedes: []
---

# Idea evolution — from ADR-only observation to DocOps

Captures the design dialogue that produced DocOps. Non-linear notes, preserved because the pushbacks are themselves design input.

## Origin

While building "Zund" (a separate agent runtime project), the author built up ~30 ADRs with rich frontmatter — `status`, `implementation`, `supersedes`, `superseded_by`, `related`, `tags`. The ADRs became the most useful doc surface in the repo for LLM consumption. Question that seeded DocOps: *can this pattern extend to task management and a broader documentation hub, as a standalone tool?*

## Original idea (since evolved)

The first `idea.md` framed DocOps as an "AI-Native GitOps Documentation & Project Management" framework with three layers: Context Engine, TypeScript Core, Web Interface. Required folders included `requirements/`, `technical/`, `tasks/`, and an auto-generated `index.md`. It described a full web UI, a `getNextTaskId` utility, `docops-lint`, etc.

That document is **aspirational inspiration** — not the spec. The spec is in these CTX docs and the ADRs that follow.

## Key pushbacks that reshaped the idea

1. **"Requirements" is the wrong name.** Stakeholder inputs are heterogeneous (PRDs, memos, interviews, strategy docs, pasted Slack threads). Formal "requirements" discipline does not fit. Renamed to **Context** with a `type:` field that accepts any project-configured shape.

2. **The `docs/` folder already in this repo is reference material from a different project**, pulled in for inspiration. Not a source of truth. Subsequent design ignores it.

3. **Dev team + LLMs only.** No web UI, no non-technical stakeholder workflows, no bidirectional SaaS sync. The audience is developers and the agents they pair with.

4. **Forget GSD-style orchestration.** GSD (installed locally) is slow in practice and too prescriptive. DocOps should give context + structure + traceability + gap detection, then stop. Let the agent's native plan-mode do the work. Pair with GStack-style role skills.

5. **CLI over MCP for phase 1.** Inside a monorepo, the agent is already in the repo; running `docops audit --json` is simpler than spinning up an MCP server. MCP can be added later for cross-repo or external-tool consumption.

6. **Bare-minimum frontmatter, then only add fields that are inevitable.** No premature schema. Three fields per doc type to start. Added fields (priority, assignee, coverage, supersedes on CTX, etc.) earned their way in by clearing a rule: "will this happen in the first 2 weeks of real use, and would lacking it push information into prose the LLM cannot query?"

7. **Calculated fields are the lever.** A two-layer design: humans write minimal source frontmatter; an indexer augments with reverse edges, resolved refs, staleness, word counts, etc. Agents read the augmented view. This saves context and makes queries O(1) instead of O(N file loads).

8. **Coverage has two kinds.** Structural gaps (detectable from the graph — ADR accepted, no task cites it) and semantic gaps (LLM judgment — tasks exist but don't actually cover the decision). Both must be first-class.

9. **Must not assume Bun or Node.** Distribution is language-agnostic — standalone binary + optional npm/pip shims. Any project in any language can adopt.

10. **Ship skills/commands for agent tooling.** GStack does this; DocOps should too. Thin Claude Code / Cursor skills that wrap CLI commands so agents get slash-command ergonomics.

## What this dialogue rejected

- Bundling phase/wave/workstream orchestration (GSD territory).
- Persona/role agents (GStack territory).
- Spec-as-contract code-generation frameworks (Kiro / Spec Kit / BMAD territory).
- Web UI at any stage of phase 1.
- Assumed runtime or package manager.
- Premature MCP investment.

## The pattern now

```
CTX (stakeholder said this)  →  ADR (we decided this)  →  TASK (do this now)
   immutable source                immutable decision        mutable work
```

Tasks cite ADRs and/or CTX. ADRs can cite CTX. The graph is the data.
