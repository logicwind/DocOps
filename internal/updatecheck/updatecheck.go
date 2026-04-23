// Package updatecheck implements a cached, lazy probe of the upstream
// docops VERSION file. It mirrors the gstack pattern: hit the network
// at most once per TTL window, fail quiet on any error, and surface
// `UPGRADE_AVAILABLE` to callers (the standalone `docops update-check`
// subcommand and the `docops upgrade` pre-flight) so users learn when
// their binary is behind upstream.
//
// The package has no project-state dependencies — it talks to the
// filesystem (cache + snooze files under ~/.docops/) and to the
// network (one HTTP GET) and that is all.
package updatecheck

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultRemoteURL is the canonical upstream version source. Override
// via Opts.RemoteURL or the DOCOPS_REMOTE_URL env var (CLI wires it).
const DefaultRemoteURL = "https://raw.githubusercontent.com/logicwind/docops/main/VERSION"

// CacheFilename and SnoozeFilename live inside the per-user state dir.
const (
	CacheFilename  = "last-update-check"
	SnoozeFilename = "update-snoozed"
)

// TTLs follow gstack's empirically chosen split: re-check up-to-date
// state often enough to surface fresh releases within a working day,
// but keep the upgrade-available reminder live for longer so users see
// it the next morning.
const (
	UpToDateTTL        = 6 * time.Hour
	UpgradeAvailableTTL = 24 * time.Hour
	DefaultTimeout     = 5 * time.Second
)

// Status enumerates the three terminal states of a single Run call.
type Status int

const (
	StatusUpToDate Status = iota
	StatusUpgradeAvailable
	StatusSkipped
)

// String renders Status for human/JSON output.
func (s Status) String() string {
	switch s {
	case StatusUpToDate:
		return "up-to-date"
	case StatusUpgradeAvailable:
		return "upgrade-available"
	case StatusSkipped:
		return "skipped"
	}
	return "unknown"
}

// Result is what Run returns. Remote is empty unless Status is
// StatusUpgradeAvailable. Reason is populated for StatusSkipped.
type Result struct {
	Status Status
	Local  string
	Remote string
	Reason string
}

// Opts configures a single update check. Zero-value safe: missing
// fields fall back to sensible defaults during Run.
type Opts struct {
	Local      string             // local version (caller-provided)
	RemoteURL  string             // defaults to DefaultRemoteURL
	StateDir   string             // defaults to $HOME/.docops
	Force      bool               // bypass cache (used by --force)
	Timeout    time.Duration      // defaults to DefaultTimeout
	Now        func() time.Time   // defaults to time.Now (test seam)
	HTTPClient *http.Client       // defaults to a client with Timeout
	Env        func(string) string // defaults to os.Getenv (test seam)
}

// versionRE validates remote and local version strings. Anything that
// does not match is treated as a network error.
var versionRE = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// devVersionMarkers identify a non-release build that should not be
// compared against upstream.
var devVersionMarkers = []string{"dev", "+dirty", "(devel)"}

// Run executes one update check. Errors are returned only for
// programming mistakes (missing Local). All transient failures
// (network errors, missing home dir, malformed cache) collapse into
// StatusSkipped or StatusUpToDate so callers never have to handle
// transient noise.
func Run(opts Opts) (Result, error) {
	if opts.Local == "" {
		return Result{}, errors.New("updatecheck: Local must be set")
	}
	applyDefaults(&opts)

	// Hard skips — never touch network or cache.
	if isDevBuild(opts.Local) {
		return Result{Status: StatusSkipped, Local: opts.Local, Reason: "dev-build"}, nil
	}
	if strings.EqualFold(opts.Env("DOCOPS_UPDATE_CHECK"), "off") {
		return Result{Status: StatusSkipped, Local: opts.Local, Reason: "disabled"}, nil
	}
	if opts.StateDir == "" {
		return Result{Status: StatusSkipped, Local: opts.Local, Reason: "no-home-dir"}, nil
	}

	// Cache fast-path.
	if !opts.Force {
		if cached, ok := readCache(opts.StateDir, opts.Local, opts.Now()); ok {
			if cached.Status == StatusUpgradeAvailable && isSnoozed(opts.StateDir, cached.Remote, opts.Now()) {
				return Result{Status: StatusSkipped, Local: opts.Local, Reason: "snoozed"}, nil
			}
			return cached, nil
		}
	}

	// Slow path: fetch remote.
	remote := fetchRemote(opts.HTTPClient, opts.RemoteURL)
	if remote == "" || !versionRE.MatchString(remote) {
		// Treat any failure as up-to-date so we don't loop on a flaky
		// upstream. Cache it briefly so the next call doesn't re-hit
		// the network either.
		_ = writeCache(opts.StateDir, fmt.Sprintf("UP_TO_DATE %s", opts.Local))
		return Result{Status: StatusUpToDate, Local: opts.Local, Reason: "offline"}, nil
	}

	if remote == opts.Local {
		_ = writeCache(opts.StateDir, fmt.Sprintf("UP_TO_DATE %s", opts.Local))
		return Result{Status: StatusUpToDate, Local: opts.Local}, nil
	}

	_ = writeCache(opts.StateDir, fmt.Sprintf("UPGRADE_AVAILABLE %s %s", opts.Local, remote))
	if isSnoozed(opts.StateDir, remote, opts.Now()) {
		return Result{Status: StatusSkipped, Local: opts.Local, Remote: remote, Reason: "snoozed"}, nil
	}
	return Result{Status: StatusUpgradeAvailable, Local: opts.Local, Remote: remote}, nil
}

// Snooze records a snooze for the given remote version. Each
// invocation bumps the level (1 → 2 → 3+), which extends the
// quiet-window. A new remote version invalidates the previous snooze
// implicitly (isSnoozed checks the version).
func Snooze(stateDir, remote string, now time.Time) error {
	if stateDir == "" {
		return errors.New("updatecheck: stateDir required to snooze")
	}
	level := 1
	if existing, err := readSnoozeFile(stateDir); err == nil && existing.version == remote {
		level = existing.level + 1
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf("%s %d %d\n", remote, level, now.Unix())
	return os.WriteFile(filepath.Join(stateDir, SnoozeFilename), []byte(body), 0o644)
}

// DefaultStateDir returns the per-user state directory ("$HOME/.docops").
// Returns "" when $HOME is unavailable; callers treat that as
// StatusSkipped.
func DefaultStateDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".docops")
}

// ───────────────────────────── internals ─────────────────────────────

func applyDefaults(o *Opts) {
	if o.RemoteURL == "" {
		o.RemoteURL = DefaultRemoteURL
	}
	if o.StateDir == "" {
		o.StateDir = DefaultStateDir()
	}
	if o.Timeout == 0 {
		o.Timeout = DefaultTimeout
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Env == nil {
		o.Env = os.Getenv
	}
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{Timeout: o.Timeout}
	}
}

func isDevBuild(v string) bool {
	for _, m := range devVersionMarkers {
		if strings.Contains(v, m) {
			return true
		}
	}
	// Also reject anything that doesn't look like x.y.z — any non-release
	// build (e.g. a goreleaser snapshot tag like "0.0.1-next") would
	// otherwise trigger a spurious upgrade prompt.
	return !versionRE.MatchString(v)
}

func fetchRemote(client *http.Client, url string) string {
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
}

// readCache loads the cache file and returns a Result if the entry is
// still within its TTL and refers to the current local version.
func readCache(stateDir, local string, now time.Time) (Result, bool) {
	path := filepath.Join(stateDir, CacheFilename)
	body, err := os.ReadFile(path)
	if err != nil {
		return Result{}, false
	}
	info, err := os.Stat(path)
	if err != nil {
		return Result{}, false
	}
	parts := strings.Fields(strings.TrimSpace(string(body)))
	if len(parts) == 0 {
		return Result{}, false
	}
	switch parts[0] {
	case "UP_TO_DATE":
		if len(parts) != 2 || parts[1] != local {
			return Result{}, false
		}
		if now.Sub(info.ModTime()) > UpToDateTTL {
			return Result{}, false
		}
		return Result{Status: StatusUpToDate, Local: local}, true
	case "UPGRADE_AVAILABLE":
		if len(parts) != 3 || parts[1] != local {
			return Result{}, false
		}
		if now.Sub(info.ModTime()) > UpgradeAvailableTTL {
			return Result{}, false
		}
		return Result{Status: StatusUpgradeAvailable, Local: local, Remote: parts[2]}, true
	}
	return Result{}, false
}

func writeCache(stateDir, line string) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stateDir, CacheFilename), []byte(line+"\n"), 0o644)
}

type snoozeEntry struct {
	version string
	level   int
	epoch   int64
}

func readSnoozeFile(stateDir string) (snoozeEntry, error) {
	body, err := os.ReadFile(filepath.Join(stateDir, SnoozeFilename))
	if err != nil {
		return snoozeEntry{}, err
	}
	parts := strings.Fields(strings.TrimSpace(string(body)))
	if len(parts) != 3 {
		return snoozeEntry{}, errors.New("malformed snooze file")
	}
	level, err := strconv.Atoi(parts[1])
	if err != nil {
		return snoozeEntry{}, err
	}
	epoch, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return snoozeEntry{}, err
	}
	return snoozeEntry{version: parts[0], level: level, epoch: epoch}, nil
}

func isSnoozed(stateDir, remote string, now time.Time) bool {
	entry, err := readSnoozeFile(stateDir)
	if err != nil {
		return false
	}
	if entry.version != remote {
		return false
	}
	var window time.Duration
	switch {
	case entry.level <= 1:
		window = 24 * time.Hour
	case entry.level == 2:
		window = 48 * time.Hour
	default:
		window = 7 * 24 * time.Hour
	}
	expires := time.Unix(entry.epoch, 0).Add(window)
	return now.Before(expires)
}
