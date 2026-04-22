---
title: CLI with --json over MCP for phase 1
status: accepted
coverage: required
date: 2026-04-22
supersedes: []
related: [ADR-0014]
tags: [distribution, agent-interface]
---

# CLI with --json over MCP for phase 1

## Context

The natural instinct for an "agent-native" tool in 2026 is MCP (Model Context Protocol): structured types, discoverable tools, persistent connection. But for a monorepo where the agent is already running inside the repo, MCP introduces ceremony without proportional benefit:

- Requires a running local server (lifecycle to manage).
- Requires host configuration (`.mcp.json`, Claude Desktop, IDE settings).
- Only works in MCP-aware hosts (Claude Code, Cursor, a few others).
- Adds a dependency and failure surface for a capability the CLI can already express.

Meanwhile, a CLI with structured (`--json`) output works in every agent, every shell, every CI, with zero setup after installation.

## Decision

Phase 1 ships only a CLI. Every command that returns data supports `--json`. No MCP server is published in phase 1.

MCP support is reserved for a later phase if and when external tools want to query DocOps state across repositories without spawning processes. Then a thin MCP server can wrap the CLI without changing the underlying semantics.

## Rationale

- Universal compatibility: Claude Code, Cursor, Aider, Codex, Windsurf, Copilot CLI, bash scripts, GitHub Actions, pre-commit hooks — all can call `docops ...` with zero config.
- Agents discover commands via `AGENTS.md` and `package.json` scripts, which they already read as part of onboarding.
- Structured output via `--json` matches what agents need (typed results) without needing a protocol.
- No daemon means no "is the server running?" failure mode.

## Consequences

- Every read command in the CLI must support `--json`. This is a testable contract.
- Help text and error messages must remain machine-parseable enough for agents.
- An MCP server can be added without changing the CLI; it just wraps the same core logic.
- This decision should be revisited if/when a cross-repo query pattern emerges. Until then, CLI is the agent interface.
