---
title: Implement update-check (cached, gstack-style) + docops update-check subcommand
status: done
priority: p2
assignee: unassigned
requires: [ADR-0023]
depends_on: []
---

## Goal

Ship the update-check mechanism specified by ADR-0023: a small
internal package, a standalone `docops update-check` subcommand, and
integration into `docops upgrade` so users learn when their binary is
behind upstream before they sync templates.

## Acceptance

### `internal/updatecheck/` package

- `Run(opts) (Result, error)` where `Result` is one of:
  - `{Status: StatusUpToDate, Local: "0.1.2"}`
  - `{Status: StatusUpgradeAvailable, Local: "0.1.1", Remote: "0.1.2"}`
  - `{Status: StatusSkipped, Reason: "snoozed" | "disabled" | "dev-build" | "offline"}`
- `Opts` fields:
  - `Local string` — caller-provided local version (from
    `internal/version`).
  - `RemoteURL string` — defaults to
    `https://raw.githubusercontent.com/logicwind/docops/main/VERSION`.
  - `StateDir string` — defaults to `$HOME/.docops`. Tests inject a
    tempdir.
  - `Force bool` — bypass cache (used by `--force`).
  - `Timeout time.Duration` — defaults to 5s.
  - `Now func() time.Time` — defaults to `time.Now`. For tests.
- Cache file `<StateDir>/last-update-check` with a single line:
  - `UP_TO_DATE <local>` (TTL 6h)
  - `UPGRADE_AVAILABLE <local> <remote>` (TTL 24h)
- Snooze file `<StateDir>/update-snoozed` with `<remote-version>
  <level> <epoch>`. Levels: 1=24h, 2=48h, 3+=7d.
  `UpdateSnooze(remote string)` helper bumps the level.
- Skip rules (return `StatusSkipped` without network I/O):
  - Local version starts with `dev` or contains `+dirty`.
  - Env `DOCOPS_UPDATE_CHECK=off`.
  - Active snooze for the matching remote version.
- Fail-quiet: any network error, timeout, or invalid response →
  write `UP_TO_DATE <local>` to cache and return `StatusUpToDate`
  with no error (so callers never see a network failure).
- Validation: remote response must match `^[0-9]+\.[0-9]+\.[0-9]+$`
  after trimming whitespace.

### `cmd/docops/cmd_update_check.go`

- Subcommand `docops update-check` with flags:
  - `--force` — bypass cache.
  - `--snooze` — record a snooze for the current available remote.
    No-op if `UP_TO_DATE`.
  - `--json` — emit `{"status": "...", "local": "...", "remote":
    "..."}`.
- Default output:
  - `UP_TO_DATE 0.1.2` to stdout, exit 0.
  - `UPGRADE_AVAILABLE 0.1.1 0.1.2` to stdout, exit 0.
  - Skipped → no output, exit 0.
- Wired into `cmd/docops/main.go` dispatch and into the top-level
  help text. `templates/skills_lint_test.go` learns the new
  subcommand and its flags.

### Integration into `docops upgrade`

- `cmd_upgrade.go` calls `updatecheck.Run` after the plan is
  computed but before the `Proceed?` prompt.
- If `StatusUpgradeAvailable`, print:
  ```
  Warning: docops 0.1.1 is installed; 0.1.2 is available.
           Run `brew upgrade docops` (or your package manager
           equivalent) before `docops upgrade`, or you'll sync the
           older templates.
  ```
  Then prompt `Continue with 0.1.1 templates anyway? [y/N]`. `--yes`
  skips the prompt (proceeds). On non-TTY stdin, the warning prints
  and we proceed (so CI is not blocked).
- `UP_TO_DATE` and `StatusSkipped` print nothing extra.

### `VERSION` file

- A new `VERSION` file at the repo root containing the current
  release (initially `0.1.2`). Goreleaser is updated (or a Makefile
  pre-release target is added) to keep this in sync with tags. If
  goreleaser config edit is non-trivial, ship the file and document
  the manual bump step in `RELEASE.md` or the README "Releasing"
  section.

### Tests

- `internal/updatecheck/updatecheck_test.go`:
  - Cache hit (UP_TO_DATE within 6h) skips network entirely (use a
    test HTTP server that fails the test if hit).
  - Cache hit (UPGRADE_AVAILABLE within 24h) returns the cached
    upgrade.
  - Stale cache triggers re-fetch.
  - Network error → `StatusUpToDate`, cache written.
  - Invalid response (HTML, empty, malformed version) → treated as
    network error.
  - Dev build → `StatusSkipped`.
  - `DOCOPS_UPDATE_CHECK=off` → `StatusSkipped`.
  - Snooze file present, version matches, within window →
    `StatusSkipped`.
  - Snooze for an older remote does not suppress a newer remote.
- `cmd/docops/cmd_update_check_test.go`: subcommand exit codes,
  output shapes (text + JSON), `--force` bypasses cache.
- `cmd/docops/cmd_upgrade_test.go` (added in TP-018) gains a case
  for the stale-binary warning path with a fake updatecheck stub.

## Notes

`internal/updatecheck/` is a leaf package — it depends only on the
standard library (`net/http`, `os`, `time`, `errors`) and on
`internal/version` for the local version string. Keep it that way;
do not import `internal/initter`, `internal/upgrader`, or anything
template-related.

`~/.docops/` is created lazily on first write. If `os.UserHomeDir()`
fails (CI without HOME set), the package returns `StatusSkipped`
with reason `"no-home-dir"` — no error to the caller.

A future ADR may broaden the integration to `docops init` (warn
first-time users they are already a release behind). Out of scope
here — TP-021 only wires `update-check` into `upgrade` and the
standalone subcommand.
