package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeDocopsRoot creates a temp dir with a valid docops.yaml and the three
// doc directories, then changes cwd into it.
func makeDocopsRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "docops.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatalf("write docops.yaml: %v", err)
	}
	for _, d := range []string{"docs/context", "docs/decisions", "docs/tasks"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	return root
}

// plantDoc writes a raw markdown doc into the given relative path under root.
func plantDoc(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

const validADR = `---
title: Some decision
status: accepted
coverage: required
date: 2026-01-01
supersedes: []
related: []
tags: []
---

# Some decision

Body.
`

const validTask = `---
title: Some task
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0001]
depends_on: []
---

# Some task

## Goal

## Acceptance

## Notes
`

const invalidTask = `---
title: Bad task
status: in-progress
priority: p2
assignee: unassigned
requires: [ADR-0001]
depends_on: []
---

# Bad task

## Goal
`

// TestCmdRefresh_HappyPath verifies a clean repo produces exit 0 and writes
// the index and state files.
func TestCmdRefresh_HappyPath(t *testing.T) {
	root := makeDocopsRoot(t)
	plantDoc(t, root, "docs/decisions/ADR-0001-some-decision.md", validADR)
	plantDoc(t, root, "docs/tasks/TP-001-some-task.md", validTask)

	code := cmdRefresh(nil)
	if code != 0 {
		t.Fatalf("cmdRefresh returned %d, want 0", code)
	}

	if _, err := os.Stat(filepath.Join(root, "docs/.index.json")); err != nil {
		t.Errorf("docs/.index.json not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/STATE.md")); err != nil {
		t.Errorf("docs/STATE.md not written: %v", err)
	}
}

// TestCmdRefresh_ValidateFailShortCircuits verifies that an invalid doc causes
// exit 1 and skips the index+state writes.
func TestCmdRefresh_ValidateFailShortCircuits(t *testing.T) {
	root := makeDocopsRoot(t)
	plantDoc(t, root, "docs/decisions/ADR-0001-some-decision.md", validADR)
	// Plant a task with an invalid status ("in-progress" is not a valid value).
	plantDoc(t, root, "docs/tasks/TP-001-bad-task.md", invalidTask)

	code := cmdRefresh(nil)
	if code != 1 {
		t.Fatalf("cmdRefresh returned %d, want 1 on validate failure", code)
	}

	// Index and state must not have been written.
	if _, err := os.Stat(filepath.Join(root, "docs/.index.json")); err == nil {
		t.Error("docs/.index.json should not exist after validate failure")
	}
	if _, err := os.Stat(filepath.Join(root, "docs/STATE.md")); err == nil {
		t.Error("docs/STATE.md should not exist after validate failure")
	}
}

// TestCmdRefresh_JSONShape verifies the --json output shape on a happy path.
func TestCmdRefresh_JSONShape(t *testing.T) {
	root := makeDocopsRoot(t)
	plantDoc(t, root, "docs/decisions/ADR-0001-some-decision.md", validADR)
	plantDoc(t, root, "docs/tasks/TP-001-some-task.md", validTask)

	// Capture stdout.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStdout := os.Stdout
	os.Stdout = w

	code := cmdRefresh([]string{"--json"})

	_ = w.Close()
	os.Stdout = origStdout

	if code != 0 {
		t.Fatalf("cmdRefresh --json returned %d, want 0", code)
	}

	buf := make([]byte, 1<<16)
	n, _ := r.Read(buf)
	raw := buf[:n]

	var out struct {
		OK    bool `json:"ok"`
		Steps []struct {
			Name    string `json:"name"`
			OK      bool   `json:"ok"`
			Skipped bool   `json:"skipped"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("--json output not valid JSON: %v\n%s", err, raw)
	}
	if !out.OK {
		t.Errorf("expected ok=true, got false")
	}
	if len(out.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(out.Steps))
	}
	names := []string{"validate", "index", "state"}
	for i, step := range out.Steps {
		if step.Name != names[i] {
			t.Errorf("step[%d].name = %q, want %q", i, step.Name, names[i])
		}
		if !step.OK {
			t.Errorf("step[%d] (%s) ok=false, want true", i, step.Name)
		}
	}
}

// TestCmdRefresh_JSONShapeOnValidateFail verifies skipped steps appear in JSON.
func TestCmdRefresh_JSONShapeOnValidateFail(t *testing.T) {
	root := makeDocopsRoot(t)
	plantDoc(t, root, "docs/decisions/ADR-0001-some-decision.md", validADR)
	plantDoc(t, root, "docs/tasks/TP-001-bad-task.md", invalidTask)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStdout := os.Stdout
	os.Stdout = w

	code := cmdRefresh([]string{"--json"})

	_ = w.Close()
	os.Stdout = origStdout

	if code != 1 {
		t.Fatalf("cmdRefresh --json returned %d, want 1", code)
	}

	buf := make([]byte, 1<<16)
	n, _ := r.Read(buf)
	raw := buf[:n]

	var out struct {
		OK    bool `json:"ok"`
		Steps []struct {
			Name    string `json:"name"`
			OK      bool   `json:"ok"`
			Skipped bool   `json:"skipped"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("--json output not valid JSON: %v\n%s", err, raw)
	}
	if out.OK {
		t.Error("expected ok=false on validate failure")
	}
	if len(out.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(out.Steps))
	}
	if out.Steps[0].Name != "validate" || out.Steps[0].OK {
		t.Errorf("step[0] should be validate/fail, got %+v", out.Steps[0])
	}
	if !out.Steps[1].Skipped {
		t.Errorf("step[1] (index) should be skipped, got %+v", out.Steps[1])
	}
	if !out.Steps[2].Skipped {
		t.Errorf("step[2] (state) should be skipped, got %+v", out.Steps[2])
	}
}

// TestCmdRefresh_NoConfig exits 2 when no docops.yaml is found.
func TestCmdRefresh_NoConfig(t *testing.T) {
	root := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	code := cmdRefresh(nil)
	if code != 2 {
		t.Errorf("expected exit 2 without docops.yaml, got %d", code)
	}
}

// TestCmdRefresh_HumanOutputContainsOK verifies the human-readable summary line.
func TestCmdRefresh_HumanOutputContainsOK(t *testing.T) {
	root := makeDocopsRoot(t)
	plantDoc(t, root, "docs/decisions/ADR-0001-some-decision.md", validADR)
	plantDoc(t, root, "docs/tasks/TP-001-some-task.md", validTask)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStdout := os.Stdout
	os.Stdout = w

	code := cmdRefresh(nil)

	_ = w.Close()
	os.Stdout = origStdout

	if code != 0 {
		t.Fatalf("cmdRefresh returned %d", code)
	}

	buf := make([]byte, 1<<16)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "docops refresh: OK") {
		t.Errorf("expected 'docops refresh: OK' in output, got:\n%s", output)
	}
}
