package updatecheck

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixedNow returns a Now() func locked to a specific instant.
func fixedNow(ts time.Time) func() time.Time {
	return func() time.Time { return ts }
}

// staticEnv returns an Env() func that ignores os.Getenv and returns
// values from a fixed map. Tests use this to set DOCOPS_UPDATE_CHECK
// without leaking into the parent process.
func staticEnv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// serveVersion stands up an httptest.Server that returns body for any
// GET. Returns the server URL and a teardown func.
func serveVersion(t *testing.T, body string) (string, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	return srv.URL, srv.Close
}

// failIfHit returns a server URL whose handler fails the test if it
// receives any request.
func failIfHit(t *testing.T) (string, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("network was hit unexpectedly: %s", r.URL.Path)
	}))
	return srv.URL, srv.Close
}

func TestRun_RequiresLocal(t *testing.T) {
	if _, err := Run(Opts{}); err == nil {
		t.Fatal("expected error when Local is empty")
	}
}

func TestRun_DevBuildSkipsWithoutNetwork(t *testing.T) {
	url, stop := failIfHit(t)
	defer stop()

	for _, local := range []string{"dev", "0.1.2+dirty", "v0.1.2-(devel)", "0.0.1-next"} {
		t.Run(local, func(t *testing.T) {
			res, err := Run(Opts{
				Local:     local,
				RemoteURL: url,
				StateDir:  t.TempDir(),
				Env:       staticEnv(nil),
			})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if res.Status != StatusSkipped || res.Reason != "dev-build" {
				t.Errorf("got %+v; want StatusSkipped/dev-build", res)
			}
		})
	}
}

func TestRun_DisabledByEnvSkipsWithoutNetwork(t *testing.T) {
	url, stop := failIfHit(t)
	defer stop()

	res, err := Run(Opts{
		Local:     "0.1.1",
		RemoteURL: url,
		StateDir:  t.TempDir(),
		Env:       staticEnv(map[string]string{"DOCOPS_UPDATE_CHECK": "off"}),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != StatusSkipped || res.Reason != "disabled" {
		t.Errorf("got %+v; want StatusSkipped/disabled", res)
	}
}

func TestRun_FreshUpToDateCacheHit_NoNetwork(t *testing.T) {
	state := t.TempDir()
	if err := os.WriteFile(filepath.Join(state, CacheFilename), []byte("UP_TO_DATE 0.1.1\n"), 0o644); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	url, stop := failIfHit(t)
	defer stop()

	res, err := Run(Opts{
		Local:     "0.1.1",
		RemoteURL: url,
		StateDir:  state,
		Now:       fixedNow(time.Now()),
		Env:       staticEnv(nil),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != StatusUpToDate {
		t.Errorf("got %+v; want StatusUpToDate", res)
	}
}

func TestRun_FreshUpgradeAvailableCacheHit_NoNetwork(t *testing.T) {
	state := t.TempDir()
	if err := os.WriteFile(filepath.Join(state, CacheFilename), []byte("UPGRADE_AVAILABLE 0.1.1 0.1.2\n"), 0o644); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	url, stop := failIfHit(t)
	defer stop()

	res, err := Run(Opts{
		Local:     "0.1.1",
		RemoteURL: url,
		StateDir:  state,
		Now:       fixedNow(time.Now()),
		Env:       staticEnv(nil),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != StatusUpgradeAvailable || res.Remote != "0.1.2" {
		t.Errorf("got %+v; want StatusUpgradeAvailable remote=0.1.2", res)
	}
}

func TestRun_StaleCacheTriggersRefetch(t *testing.T) {
	state := t.TempDir()
	cachePath := filepath.Join(state, CacheFilename)
	if err := os.WriteFile(cachePath, []byte("UP_TO_DATE 0.1.1\n"), 0o644); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	// Force the cache to look 12 hours old (UP_TO_DATE TTL is 6h).
	old := time.Now().Add(-12 * time.Hour)
	if err := os.Chtimes(cachePath, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	url, stop := serveVersion(t, "0.1.2\n")
	defer stop()

	res, err := Run(Opts{
		Local:     "0.1.1",
		RemoteURL: url,
		StateDir:  state,
		Now:       fixedNow(time.Now()),
		Env:       staticEnv(nil),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != StatusUpgradeAvailable || res.Remote != "0.1.2" {
		t.Errorf("got %+v; want StatusUpgradeAvailable remote=0.1.2", res)
	}
}

func TestRun_NetworkErrorYieldsUpToDate(t *testing.T) {
	// Point at an address with no listener — http.Client.Get will fail
	// with a connection refused.
	res, err := Run(Opts{
		Local:     "0.1.1",
		RemoteURL: "http://127.0.0.1:1/VERSION",
		StateDir:  t.TempDir(),
		Timeout:   200 * time.Millisecond,
		Now:       fixedNow(time.Now()),
		Env:       staticEnv(nil),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != StatusUpToDate {
		t.Errorf("got %+v; want StatusUpToDate on network error", res)
	}
}

func TestRun_InvalidRemoteResponseYieldsUpToDate(t *testing.T) {
	cases := map[string]string{
		"empty":      "",
		"html":       "<html><body>404</body></html>",
		"prefixed":   "v0.1.2",
		"too-short":  "0.1",
		"with-space": "0.1.2 extra",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			url, stop := serveVersion(t, body)
			defer stop()
			res, err := Run(Opts{
				Local:     "0.1.1",
				RemoteURL: url,
				StateDir:  t.TempDir(),
				Now:       fixedNow(time.Now()),
				Env:       staticEnv(nil),
			})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if res.Status != StatusUpToDate {
				t.Errorf("body=%q got %+v; want StatusUpToDate", body, res)
			}
		})
	}
}

func TestRun_UpToDateMatchingRemote(t *testing.T) {
	url, stop := serveVersion(t, "0.1.1\n")
	defer stop()
	state := t.TempDir()
	res, err := Run(Opts{
		Local:     "0.1.1",
		RemoteURL: url,
		StateDir:  state,
		Now:       fixedNow(time.Now()),
		Env:       staticEnv(nil),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != StatusUpToDate {
		t.Errorf("got %+v; want StatusUpToDate", res)
	}
	cached, err := os.ReadFile(filepath.Join(state, CacheFilename))
	if err != nil || string(cached) != "UP_TO_DATE 0.1.1\n" {
		t.Errorf("cache file = %q (err=%v); want %q", cached, err, "UP_TO_DATE 0.1.1\n")
	}
}

func TestRun_SnoozeSuppressesMatchingRemote(t *testing.T) {
	state := t.TempDir()
	now := time.Now()
	if err := os.WriteFile(
		filepath.Join(state, SnoozeFilename),
		[]byte(fmt.Sprintf("0.1.2 1 %d\n", now.Unix())),
		0o644,
	); err != nil {
		t.Fatalf("seed snooze: %v", err)
	}

	url, stop := serveVersion(t, "0.1.2\n")
	defer stop()
	res, err := Run(Opts{
		Local:     "0.1.1",
		RemoteURL: url,
		StateDir:  state,
		Now:       fixedNow(now),
		Env:       staticEnv(nil),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != StatusSkipped || res.Reason != "snoozed" {
		t.Errorf("got %+v; want StatusSkipped/snoozed", res)
	}
}

func TestRun_SnoozeForOlderRemoteDoesNotSuppressNewer(t *testing.T) {
	state := t.TempDir()
	now := time.Now()
	// Snooze 0.1.2; remote will offer 0.1.3 — should NOT be suppressed.
	if err := os.WriteFile(
		filepath.Join(state, SnoozeFilename),
		[]byte(fmt.Sprintf("0.1.2 1 %d\n", now.Unix())),
		0o644,
	); err != nil {
		t.Fatalf("seed snooze: %v", err)
	}

	url, stop := serveVersion(t, "0.1.3\n")
	defer stop()
	res, err := Run(Opts{
		Local:     "0.1.1",
		RemoteURL: url,
		StateDir:  state,
		Now:       fixedNow(now),
		Env:       staticEnv(nil),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != StatusUpgradeAvailable || res.Remote != "0.1.3" {
		t.Errorf("got %+v; want StatusUpgradeAvailable remote=0.1.3", res)
	}
}

func TestRun_ExpiredSnoozeNoLongerSuppresses(t *testing.T) {
	state := t.TempDir()
	now := time.Now()
	// level 1 = 24h; snooze stamped 48h ago is expired.
	stamped := now.Add(-48 * time.Hour).Unix()
	if err := os.WriteFile(
		filepath.Join(state, SnoozeFilename),
		[]byte(fmt.Sprintf("0.1.2 1 %d\n", stamped)),
		0o644,
	); err != nil {
		t.Fatalf("seed snooze: %v", err)
	}

	url, stop := serveVersion(t, "0.1.2\n")
	defer stop()
	res, err := Run(Opts{
		Local:     "0.1.1",
		RemoteURL: url,
		StateDir:  state,
		Now:       fixedNow(now),
		Env:       staticEnv(nil),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != StatusUpgradeAvailable {
		t.Errorf("got %+v; want StatusUpgradeAvailable after snooze expired", res)
	}
}

func TestRun_ForceBypassesCache(t *testing.T) {
	state := t.TempDir()
	if err := os.WriteFile(filepath.Join(state, CacheFilename), []byte("UP_TO_DATE 0.1.1\n"), 0o644); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	url, stop := serveVersion(t, "0.1.2\n")
	defer stop()

	res, err := Run(Opts{
		Local:     "0.1.1",
		RemoteURL: url,
		StateDir:  state,
		Force:     true,
		Now:       fixedNow(time.Now()),
		Env:       staticEnv(nil),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != StatusUpgradeAvailable || res.Remote != "0.1.2" {
		t.Errorf("got %+v; want StatusUpgradeAvailable remote=0.1.2", res)
	}
}

func TestSnooze_BumpsLevelOnRepeatForSameRemote(t *testing.T) {
	state := t.TempDir()
	now := time.Now()
	if err := Snooze(state, "0.1.2", now); err != nil {
		t.Fatalf("first snooze: %v", err)
	}
	if err := Snooze(state, "0.1.2", now.Add(time.Minute)); err != nil {
		t.Fatalf("second snooze: %v", err)
	}
	entry, err := readSnoozeFile(state)
	if err != nil {
		t.Fatalf("readSnoozeFile: %v", err)
	}
	if entry.level != 2 {
		t.Errorf("level = %d; want 2 after second snooze of same remote", entry.level)
	}
}

func TestSnooze_ResetsLevelOnNewRemote(t *testing.T) {
	state := t.TempDir()
	now := time.Now()
	if err := Snooze(state, "0.1.2", now); err != nil {
		t.Fatalf("first snooze: %v", err)
	}
	if err := Snooze(state, "0.1.3", now); err != nil {
		t.Fatalf("snooze for new remote: %v", err)
	}
	entry, err := readSnoozeFile(state)
	if err != nil {
		t.Fatalf("readSnoozeFile: %v", err)
	}
	if entry.version != "0.1.3" || entry.level != 1 {
		t.Errorf("entry = %+v; want version=0.1.3 level=1", entry)
	}
}
