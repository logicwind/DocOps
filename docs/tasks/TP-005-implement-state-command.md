---
title: Implement `docops state` — generate STATE.md
status: done
priority: p1
assignee: claude
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

## Outcome (2026-04-22)

- **Package:** `internal/state` (`state.go`, `render.go`, `git.go`) plus `cmd/docops/cmd_state.go`. Dispatcher wiring in `main.go` (also covers TP-006).
- **Pipeline:** `docops state` runs `validate` first and refuses to generate from a broken repo, then builds the index **in-memory** (rather than reading `.index.json` from disk) so state is always fresh. Recent activity pulls from `git log --since=<window> --pretty=format:"%cI%x09%s%x09%H"`; if git is unavailable the fallback derives the same fields from `last_touched` in the index.
- **Sections:** Header, Counts (11 counters per kind), Needs attention (5 structural rules driven by `cfg.Gaps` thresholds + severity), Active work (sorted by ID, formats `TP-XXX (assignee, priority) title — requires: ...`), Recent activity (cap 20, newest-first, dedup).
- **CLI contract:** `--stdout` prints and does not write; `--json` emits the structured `Snapshot`; default writes to `cfg.Paths.State` (default `docs/STATE.md`). Exit 0/1/2.
- **Config:** added `recent_activity_window_days` (default 7) at the top level of `docops.yaml` with full default-filling in `config.ApplyDefaults`.
- **Determinism:** two successive `Compute`s with the same inputs produce equal `Snapshot`s (excluding `GeneratedAt`); `Markdown()` is byte-stable.
- **Tests:** 39 total in `internal/state` — all 5 needs-attention rules at-threshold / below-threshold, severity normalisation (warn / error / off), sort order (severity then ID), active-work markdown permutations (with/without assignee/priority/requires), recent-activity git-wins / fallback / out-of-window / cap-at-20, JSON validity, determinism, dog-food pass over the live repo.
- **Dog-food:** `./bin/docops state --stdout` produces the canonical STATE.md for this repo; the committed `docs/STATE.md` is now produced by the command, replacing the hand-maintained bootstrap version.
