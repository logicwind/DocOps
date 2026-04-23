---
title: Ship CLAUDE.md alongside AGENTS.md — both share the docops block
status: draft
coverage: required
date: "2026-04-23"
supersedes: []
related: [ADR-0021, ADR-0023]
tags: []
---

## Context

DocOps writes its invariants (citation rules, no-edit-STATE.md, schema
locations, etc.) into a delimited block in `AGENTS.md` so any coding
agent — Claude Code, Cursor, Codex, Aider, Copilot, Windsurf, Zed —
finds them on first contact. AGENTS.md was chosen as the multi-tool
standard surface.

The gap: **Claude Code reads `CLAUDE.md` by default, not `AGENTS.md`.**
A user who runs `docops init` and then opens the project in Claude
Code never sees the docops invariants until they manually point Claude
at AGENTS.md. The invariants are then routinely violated (tasks
without citations, edits to STATE.md, etc.) and the user blames docops.

`gstack` solves this by treating `CLAUDE.md` as the canonical
single-source and rendering host-specific files (AGENTS.md for
Hermes, etc.) via a path/tool rewrite map. That works because gstack
ships per-host packaging anyway. DocOps does not have host packaging
and does not want to grow one for this single use case.

## Decision

DocOps writes **both** `CLAUDE.md` and `AGENTS.md` at the project
root. Both contain the same `<!-- docops:start --> … <!-- docops:end -->`
block with the same docops invariants. The non-block preamble may
differ (CLAUDE.md gets a one-line note that AGENTS.md is the
multi-tool sibling; otherwise content is identical).

The block-merge logic that already powers `init` and `upgrade` (see
ADR-0021, `internal/scaffold.MergeAgentsBlock`) is reused verbatim
for CLAUDE.md. The same three cases apply:

- File absent → write the full template.
- File present without block → append the block, preserve user content.
- File present with block → refresh just the block, preserve everything outside the markers.

## Rationale

- **Smallest delta over current behavior.** No template engine, no
  host registry, no per-host rendering — just a second template and
  a second call to the existing planner.
- **Robust against Claude Code defaults.** The user no longer has
  to know to redirect Claude at AGENTS.md.
- **Multi-tool ecosystem unaffected.** AGENTS.md keeps shipping; tools
  that read it (Cursor, Aider, Codex) see no change.
- **Symlink rejected.** A CLAUDE.md → AGENTS.md symlink would skirt
  the duplication problem but breaks on Windows without dev mode and
  has poor git portability across shell environments.
- **Flipping canonical (gstack-style) rejected.** Making CLAUDE.md
  primary and AGENTS.md derived would force a host registry and a
  template-rewrite engine into docops. Out of scope for the value.
- **Block duplication is bounded.** The docops block is ~80 lines.
  Both files refresh in one upgrade pass; nothing drifts independently
  because the source of truth is the embedded template, not either
  file.

## Consequences

- `docops init` now creates two files where it used to create one.
  Existing v0.1.x projects that already have AGENTS.md but not
  CLAUDE.md get CLAUDE.md added on next `docops upgrade` (the planner
  treats "absent" as "create"; no flag required).
- Users who hand-curate CLAUDE.md keep their content — only the
  delimited block is owned by docops.
- The `templates/AGENTS.md.tmpl` and `templates/CLAUDE.md.tmpl` files
  must stay in sync for the docops block. A small templates-package
  test asserts that `ExtractBlock` returns identical content from
  both, so future drift is caught at build time.
- The "Notes for humans" footer on each file may eventually diverge
  per audience. ADR remains valid; only the block contract is
  load-bearing.
- A future ADR may extend the same pattern to `.cursorrules`,
  `.aider.conf.yml`, or other tool-specific surfaces. Each would be
  evaluated on its own ROI; this ADR commits only to AGENTS.md +
  CLAUDE.md.
