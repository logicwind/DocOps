---
title: Update-check — cached, lazy, opt-out version probe
status: draft
coverage: required
date: "2026-04-23"
supersedes: []
related: [ADR-0021, ADR-0019]
tags: []
---

## Context

DocOps ships through Homebrew, Scoop, and direct binary downloads. Once
a user runs `docops init`, their project files (skills, schemas,
AGENTS.md block) drift from upstream every time we cut a release that
touches templates. ADR-0021 / TP-018 introduce `docops upgrade` to sync
those files in place — but `docops upgrade` only knows about the binary
the user already has installed. If their binary is itself stale (e.g.
the user is on `docops v0.1.1` but `v0.1.2` shipped two weeks ago with
new skill files), `docops upgrade` will quietly install the v0.1.1
templates and the user will think they are current.

The user has no in-band way to learn that their binary is behind. The
fix is to add a periodic update check, the same way `gstack` does:
cache the result in `~/.docops/`, only hit the network when the cache
is stale, fail quiet, and surface a one-line reminder in commands that
naturally care (`upgrade` first, others later).

## Decision

Ship a small `internal/updatecheck/` package and one new subcommand
`docops update-check`. Wire the package into `docops upgrade` so a
stale binary surfaces before any in-place template sync runs.

### Contract

`docops update-check` prints exactly one of:

- `UP_TO_DATE <local-version>` — local matches remote.
- `UPGRADE_AVAILABLE <local-version> <remote-version>` — remote ahead.
- (nothing) — check skipped (snoozed, disabled, offline, dev build).

Exit code is always `0`. The output line is intentionally
shell-parseable; this matches gstack and lets users wire it into shell
prompts, MOTDs, or CI guardrails.

### Caching

Cache file: `~/.docops/last-update-check`. Format: the same one-line
shape as the `update-check` output above (`UP_TO_DATE <ver>` or
`UPGRADE_AVAILABLE <old> <new>`). Two TTLs:

- `UP_TO_DATE`: **6 hours**. We re-check periodically so newly
  shipped releases surface within a working day.
- `UPGRADE_AVAILABLE`: **24 hours**. The user already knows; we
  don't need to re-fetch as eagerly, but we do want to keep nagging
  on the next day's first invocation.

Cache is per-user, not per-project. Multiple projects share one cache
file because the binary is global.

### Remote source

Primary: `https://raw.githubusercontent.com/logicwind/docops/main/VERSION`.
A `VERSION` file is added to the repo root and bumped during release
(or derived from the last git tag — implementer's call, see TP-021).
Validation: response must match `^[0-9]+\.[0-9]+\.[0-9]+$`. Anything
else is treated as a network error and the cache is updated with
`UP_TO_DATE` so we don't loop on transient failures.

Timeout: 5 seconds. On any network error or invalid response, fail
silently and write `UP_TO_DATE <local>` to the cache so the user is
never blocked or spammed by an unreachable upstream.

### Snooze

A user can snooze a specific available upgrade by writing
`~/.docops/update-snoozed` with `<remote-version> <level> <epoch>`
(matches gstack's format). Levels: 1=24h, 2=48h, 3+=7d. A new remote
version invalidates any active snooze. The snooze file is written by
`docops update-check --snooze` (level auto-increments each invocation).

### Opt-out

Two ways to disable:

- Per-user: `~/.docops/update-snoozed` containing `disabled` (or any
  invalid version string with `level=999`). Treated as permanent
  snooze.
- Per-invocation env: `DOCOPS_UPDATE_CHECK=off` skips the check.
  Honored by both the standalone subcommand and the internal
  piggyback in `docops upgrade`.

Dev builds (version starts with `dev` or contains `+dirty`) skip the
check unconditionally — there is no meaningful "remote" for them.

### Integration with `docops upgrade`

Before printing the upgrade plan, `docops upgrade` calls
`updatecheck.Run()`. If it returns `UPGRADE_AVAILABLE`, we print a
warning block:

```
Warning: docops v0.1.1 is installed; v0.1.2 is available.
         Run `brew upgrade docops` (or your package manager's
         equivalent) before `docops upgrade`, or you'll sync the
         older templates.

Continue with v0.1.1 templates anyway? [y/N]
```

`--yes` skips the warning prompt the same way it skips the main
confirm. `UP_TO_DATE` and silent results print nothing.

No other subcommand fires the check automatically. Scripts that pipe
`docops list` / `docops get` / `docops search` should not pay a
network or stat-syscall cost they didn't ask for. Users who want
proactive nagging can run `docops update-check` from their shell
profile.

## Rationale

- A standalone subcommand keeps the behavior auditable and scriptable
  — power users can opt in to extra nagging without us baking it into
  every code path.
- Caching with multi-hour TTLs means `docops upgrade` stays
  near-instant on warm cache and at most one 5s network hit on cold
  cache. The two-tier TTL (shorter for up-to-date, longer for
  upgrade-available) mirrors gstack's empirically-chosen pattern.
- Failing quiet on network errors is non-negotiable: docops runs in
  pre-commit hooks and CI environments where flaky DNS would
  otherwise break commits.
- Hooking only into `docops upgrade` and not every command keeps the
  blast radius small. We can add more integration points later if
  users ask for them.
- `~/.docops/` as a state dir mirrors `~/.gstack/`, `~/.aws/`,
  `~/.config/gh/`. No XDG variant for v1; if users object, we can
  honor `$XDG_STATE_HOME` in a follow-up.

## Consequences

- A new state directory `~/.docops/` is created lazily the first time
  `update-check` runs. It must not be required for any other docops
  command to function — packaging and CI environments without a
  writable home directory must still work.
- The repository now ships a `VERSION` file at the root that must be
  kept in sync with release tags. Goreleaser config gets a step (or
  the maintainer remembers — TP-021 picks the cheaper option).
- `docops upgrade` gains an interactive prompt path that fires when
  the binary is stale. CI must always pass `--yes` to avoid hanging.
- A future ADR may extend update-check to fire from `docops init`
  (so first-time users learn they're already a release behind), but
  that is out of scope here.
- Telemetry: we deliberately do **not** ping any upstream beyond the
  one raw-content fetch. No counts, no UA, no install ID. If we want
  install metrics later, that needs its own ADR.
