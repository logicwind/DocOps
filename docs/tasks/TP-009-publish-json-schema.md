---
title: Publish JSON Schema for editor tooling
status: done
priority: p2
assignee: unassigned
requires: [ADR-0002]
depends_on: [TP-002]
---

# Publish JSON Schema for editor tooling

## Goal

Emit the frontmatter schemas as JSON Schema so editors (VS Code YAML plugin, JetBrains, Zed) can validate in-editor without running the CLI.

## Acceptance

- Three JSON Schema files under `docs/.docops/schema/`:
  - `context.schema.json`
  - `decision.schema.json`
  - `task.schema.json`
- `docops init` writes them; subsequent schema changes regenerate them.
- Schema draft: 2020-12 (widely supported).
- VS Code `settings.json` snippet in README showing how to wire it up:
  ```json
  "yaml.schemas": {
    "./docs/.docops/schema/task.schema.json": "docs/tasks/*.md"
  }
  ```
- Schemas expose `type:` enum on CTX dynamically via `docops.yaml` — regenerate on config changes.
- Published URL (optional, phase 2): hosted on `schemas.docops.dev/v1/...` for reference from remote editors.

## Notes

This is the smallest-effort, highest-ergonomic-value phase-1 task. Agents get in-editor red squiggles for invalid frontmatter without installing DocOps.
