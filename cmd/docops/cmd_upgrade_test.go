package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/logicwind/docops/internal/scaffold"
	"github.com/logicwind/docops/templates"
)

// upgradeFixture builds a minimal docops-initialized tempdir, chdirs
// into it for the test, and returns the absolute path. cmdUpgrade
// reads cwd, so the chdir is what couples it to the fixture.
func upgradeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	yamlBody, err := templates.DocopsYAML()
	if err != nil {
		t.Fatalf("template DocopsYAML: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docops.yaml"), yamlBody, 0o644); err != nil {
		t.Fatalf("write docops.yaml: %v", err)
	}

	skills, err := scaffold.LoadShippedSkills()
	if err != nil {
		t.Fatalf("LoadShippedSkills: %v", err)
	}
	for _, dir := range []string{".claude/skills/docops", ".cursor/commands/docops"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		for name, body := range skills {
			if err := os.WriteFile(filepath.Join(root, dir, name), body, 0o644); err != nil {
				t.Fatalf("seed skill: %v", err)
			}
		}
	}

	prevDir, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir fixture: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevDir) })

	return root
}

func TestCmdUpgrade_RefusesWithoutDocopsYAML(t *testing.T) {
	root := t.TempDir()
	prevDir, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevDir) })

	code := cmdUpgrade([]string{"--dry-run"})
	if code != 2 {
		t.Errorf("exit = %d; want 2 when docops.yaml is missing", code)
	}
}

func TestCmdUpgrade_DryRunWritesNothing(t *testing.T) {
	root := upgradeFixture(t)
	stale := filepath.Join(root, ".claude/skills/docops/old-command.md")
	if err := os.WriteFile(stale, []byte("stale\n"), 0o644); err != nil {
		t.Fatalf("seed stale: %v", err)
	}
	code := cmdUpgrade([]string{"--dry-run"})
	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}
	if _, err := os.Stat(stale); err != nil {
		t.Errorf("dry-run should not delete stale skill: %v", err)
	}
}

func TestCmdUpgrade_JSONShape(t *testing.T) {
	upgradeFixture(t)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	prevStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = prevStdout })

	code := cmdUpgrade([]string{"--dry-run", "--json"})
	w.Close()
	if code != 0 {
		t.Fatalf("exit = %d; want 0", code)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read: %v", err)
	}

	var payload struct {
		OK      bool `json:"ok"`
		Actions []struct {
			Path string `json:"path"`
			Kind string `json:"kind"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\nbody=%s", err, buf.String())
	}
	if !payload.OK {
		t.Errorf("payload.OK = false")
	}
	if len(payload.Actions) == 0 {
		t.Errorf("expected non-empty actions list")
	}
	allowed := map[string]bool{"new": true, "refreshed": true, "removed": true, "up-to-date": true, "block-refreshed": true}
	for _, a := range payload.Actions {
		if !allowed[a.Kind] {
			t.Errorf("unexpected action kind %q for %s", a.Kind, a.Path)
		}
	}
}

func TestCmdUpgrade_YesFlagSkipsPromptAndApplies(t *testing.T) {
	root := upgradeFixture(t)
	stale := filepath.Join(root, ".claude/skills/docops/old-command.md")
	if err := os.WriteFile(stale, []byte("stale\n"), 0o644); err != nil {
		t.Fatalf("seed stale: %v", err)
	}

	// Redirect stdout so the (potentially non-tty) prompt and plan
	// don't pollute test output. We can't actually drive the prompt
	// from a test without a TTY, but --yes bypasses the prompt
	// entirely so the apply runs unconditionally.
	r, w, _ := os.Pipe()
	prevStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = prevStdout })

	code := cmdUpgrade([]string{"--yes"})
	w.Close()
	if code != 0 {
		t.Errorf("exit = %d; want 0", code)
	}

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("--yes should have applied the upgrade and removed the stale skill: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()
	if !strings.Contains(out, "docops upgrade:") {
		t.Errorf("expected output summary line; got %q", out)
	}
}
