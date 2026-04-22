---
title: Implement `docops state` — generate STATE.md
status: backlog
priority: p1
assignee: unassigned
requires: [ADR-0007, ADR-0008]
depends_on: [TP-004]
---

# Implement `docops state` — generate STATE.md

## Goal

Command that reads `.index.json` and writes `docs/STATE.md` — the human- and LLM-readable snapshot described in ADR-0007.

## Acceptance

- Regenerates `docs/STATE.md` with the five required sections in order: Header, Counts, Needs attention, Active work, Recent activity.
- "Needs attention" applies rules from ADR-0008 and `docops.yaml`. Each bullet names the doc, the reason, and an implied next action.
- "Active work" lists tasks with `status: active`, formatted per ADR-0007.
- "Recent activity" lists doc changes within the configured window (default 7 days), sourced from git log touching the docs folders.
- `--stdout` flag prints the content without writing the file (useful for agents querying state without disk mutation).
- `--json` flag emits the same content as structured data.
- Output is deterministic for the same `.index.json` input.

## Notes

STATE.md is meant to be committed. Its regeneration is a normal part of the index pipeline.
