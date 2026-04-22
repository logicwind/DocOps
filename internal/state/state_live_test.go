package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/index"
	"github.com/logicwind/docops/internal/loader"
)

// findProjectRoot walks up from the working directory looking for docops.yaml.
func findProjectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "docops.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("docops.yaml not found walking up from", dir)
		}
		dir = parent
	}
}

// TestState_LiveDogfood runs Compute over the actual docs/ directory. It does
// not write anything to disk; it only exercises the return values.
func TestState_LiveDogfood(t *testing.T) {
	root := findProjectRoot(t)

	cfg, _, err := config.FindAndLoad(root)
	if err != nil {
		t.Fatalf("FindAndLoad: %v", err)
	}

	set, err := loader.Load(root, cfg)
	if err != nil {
		t.Fatalf("loader.Load: %v", err)
	}

	now := time.Now()
	idx, err := index.Build(set, cfg, root, now)
	if err != nil {
		t.Fatalf("index.Build: %v", err)
	}

	snap := Compute(idx, cfg, nil, now)

	// Must not panic and must contain at least some docs.
	total := snap.Counts.ContextActive + snap.Counts.ContextSuperseded +
		snap.Counts.ADRAccepted + snap.Counts.ADRDraft + snap.Counts.ADRSuperseded +
		snap.Counts.TaskBacklog + snap.Counts.TaskActive + snap.Counts.TaskBlocked + snap.Counts.TaskDone
	if total == 0 {
		t.Error("Compute returned zero total document counts on live repo")
	}

	md := snap.Markdown()
	if len(md) == 0 {
		t.Error("Markdown() returned empty string")
	}
	for _, section := range []string{"## Counts", "## Needs attention", "## Active work", "## Recent activity"} {
		if !strings.Contains(md, section) {
			t.Errorf("Markdown() missing section %q", section)
		}
	}

	out, err := snap.JSON()
	if err != nil {
		t.Fatalf("JSON() error: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Errorf("JSON() output is not valid JSON: %v", err)
	}
}
