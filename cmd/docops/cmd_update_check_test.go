package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdUpdateCheck_UpToDateLine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "0.1.1\n")
	}))
	defer srv.Close()
	t.Setenv("DOCOPS_REMOTE_URL", srv.URL)
	t.Setenv("DOCOPS_UPDATE_CHECK", "")

	state := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runUpdateCheck(nil, &stdout, &stderr, "0.1.1", state)
	if code != 0 {
		t.Fatalf("exit = %d; want 0; stderr=%s", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "UP_TO_DATE 0.1.1" {
		t.Errorf("stdout = %q; want %q", got, "UP_TO_DATE 0.1.1")
	}
}

func TestCmdUpdateCheck_UpgradeAvailableLine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "0.1.2\n")
	}))
	defer srv.Close()
	t.Setenv("DOCOPS_REMOTE_URL", srv.URL)
	t.Setenv("DOCOPS_UPDATE_CHECK", "")

	state := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runUpdateCheck(nil, &stdout, &stderr, "0.1.1", state)
	if code != 0 {
		t.Fatalf("exit = %d; want 0; stderr=%s", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "UPGRADE_AVAILABLE 0.1.1 0.1.2" {
		t.Errorf("stdout = %q; want %q", got, "UPGRADE_AVAILABLE 0.1.1 0.1.2")
	}
}

func TestCmdUpdateCheck_DisabledIsSilent(t *testing.T) {
	t.Setenv("DOCOPS_REMOTE_URL", "http://127.0.0.1:1/never-hit")
	t.Setenv("DOCOPS_UPDATE_CHECK", "off")

	state := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runUpdateCheck(nil, &stdout, &stderr, "0.1.1", state)
	if code != 0 {
		t.Fatalf("exit = %d; want 0", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q; want empty when disabled", stdout.String())
	}
}

func TestCmdUpdateCheck_DevBuildIsSilent(t *testing.T) {
	t.Setenv("DOCOPS_REMOTE_URL", "http://127.0.0.1:1/never-hit")
	t.Setenv("DOCOPS_UPDATE_CHECK", "")

	state := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runUpdateCheck(nil, &stdout, &stderr, "dev", state)
	if code != 0 {
		t.Fatalf("exit = %d; want 0", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q; want empty for dev build", stdout.String())
	}
}

func TestCmdUpdateCheck_JSONShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "0.1.2\n")
	}))
	defer srv.Close()
	t.Setenv("DOCOPS_REMOTE_URL", srv.URL)
	t.Setenv("DOCOPS_UPDATE_CHECK", "")

	state := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runUpdateCheck([]string{"--json"}, &stdout, &stderr, "0.1.1", state)
	if code != 0 {
		t.Fatalf("exit = %d; want 0; stderr=%s", code, stderr.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\nbody=%s", err, stdout.String())
	}
	if payload["status"] != "upgrade-available" || payload["local"] != "0.1.1" || payload["remote"] != "0.1.2" {
		t.Errorf("payload = %v; missing/wrong fields", payload)
	}
}

func TestCmdUpdateCheck_ForceBypassesCache(t *testing.T) {
	state := t.TempDir()
	if err := os.WriteFile(filepath.Join(state, "last-update-check"), []byte("UP_TO_DATE 0.1.1\n"), 0o644); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "0.1.2\n")
	}))
	defer srv.Close()
	t.Setenv("DOCOPS_REMOTE_URL", srv.URL)
	t.Setenv("DOCOPS_UPDATE_CHECK", "")

	var stdout, stderr bytes.Buffer
	code := runUpdateCheck([]string{"--force"}, &stdout, &stderr, "0.1.1", state)
	if code != 0 {
		t.Fatalf("exit = %d; want 0; stderr=%s", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "UPGRADE_AVAILABLE 0.1.1 0.1.2" {
		t.Errorf("stdout = %q; want fresh upgrade line", got)
	}
}

func TestCmdUpdateCheck_SnoozeWritesFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "0.1.2\n")
	}))
	defer srv.Close()
	t.Setenv("DOCOPS_REMOTE_URL", srv.URL)
	t.Setenv("DOCOPS_UPDATE_CHECK", "")

	state := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runUpdateCheck([]string{"--snooze"}, &stdout, &stderr, "0.1.1", state)
	if code != 0 {
		t.Fatalf("exit = %d; want 0; stderr=%s", code, stderr.String())
	}
	body, err := os.ReadFile(filepath.Join(state, "update-snoozed"))
	if err != nil {
		t.Fatalf("snooze file missing: %v", err)
	}
	if !strings.HasPrefix(string(body), "0.1.2 1 ") {
		t.Errorf("snooze body = %q; want start with '0.1.2 1 '", body)
	}
}
