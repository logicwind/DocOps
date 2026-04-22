package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nachiket/docops/internal/config"
	"github.com/nachiket/docops/internal/index"
	"github.com/nachiket/docops/internal/loader"
	"github.com/nachiket/docops/internal/validator"
)

// findProjectRoot walks up from the test file's directory to find docops.yaml.
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

// TestAudit_DogFood runs Audit over the real repo docs/ and checks the report
// is well-formed, Human/JSON produce output, and the --include-not-needed path
// emits at least one info finding (ADR-0014 has coverage: not-needed).
func TestAudit_DogFood(t *testing.T) {
	root := findProjectRoot(t)

	cfg, _, err := config.FindAndLoad(root)
	if err != nil {
		t.Fatalf("FindAndLoad: %v", err)
	}

	set, err := loader.Load(root, cfg)
	if err != nil {
		t.Fatalf("loader.Load: %v", err)
	}

	vreport := validator.Validate(set, cfg)
	if !vreport.OK() {
		t.Fatalf("repo validation failed (%d errors); fix before auditing", len(vreport.Errors))
	}

	idx, err := index.Build(set, cfg, root, time.Now())
	if err != nil {
		t.Fatalf("index.Build: %v", err)
	}

	// Audit without includeNotNeeded.
	r1 := Audit(idx, set, cfg, false)
	if r1 == nil {
		t.Fatal("Audit returned nil")
	}
	h1 := r1.Human()
	if h1 == "" {
		t.Error("Human() returned empty string")
	}
	j1, err := r1.JSON()
	if err != nil {
		t.Fatalf("JSON() error: %v", err)
	}
	if len(j1) == 0 {
		t.Error("JSON() returned empty bytes")
	}

	// Audit with includeNotNeeded=true — must produce at least one info finding.
	r2 := Audit(idx, set, cfg, true)
	if r2 == nil {
		t.Fatal("Audit(includeNotNeeded=true) returned nil")
	}
	h2 := r2.Human()
	if h2 == "" {
		t.Error("Human() (includeNotNeeded) returned empty string")
	}
	j2, err := r2.JSON()
	if err != nil {
		t.Fatalf("JSON() (includeNotNeeded) error: %v", err)
	}
	if len(j2) == 0 {
		t.Error("JSON() (includeNotNeeded) returned empty bytes")
	}

	// There is at least one ADR with coverage:not-needed in the repo (ADR-0014),
	// so includeNotNeeded=true must yield at least one info finding.
	infos := filterBySev(r2.Findings, "info")
	if len(infos) == 0 {
		t.Error("expected at least one info finding with includeNotNeeded=true (ADR-0014 has coverage:not-needed)")
	}

	// Human output for the includeNotNeeded case should contain the Info section.
	if !strings.Contains(h2, "Info") {
		t.Errorf("Human() with info findings should contain 'Info' section; got:\n%s", h2)
	}

	// Both runs must produce identical output when called twice (determinism).
	r3 := Audit(idx, set, cfg, true)
	h3 := r3.Human()
	if h2 != h3 {
		t.Error("Human() output differs across two consecutive Audit calls on the same input")
	}
	j3, _ := r3.JSON()
	if string(j2) != string(j3) {
		t.Error("JSON() output differs across two consecutive Audit calls on the same input")
	}
}
