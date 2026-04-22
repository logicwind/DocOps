package initter

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/nachiket/docops/internal/config"
	"github.com/nachiket/docops/internal/loader"
	"github.com/nachiket/docops/internal/validator"
)

// TestRun_BareRepo_PassesValidate proves the acceptance criterion from
// TP-007: "It must succeed on a brand-new repo with only a `.git` folder."
// After init, loading + validating the scaffolded tree should produce
// zero errors (the three folders are present, docops.yaml parses, and
// no stale frontmatter exists to trip a rule).
func TestRun_BareRepo_PassesValidate(t *testing.T) {
	root := t.TempDir()
	withGit(t, root)

	if _, err := Run(Options{Root: root, Out: io.Discard}); err != nil {
		t.Fatalf("init: %v", err)
	}

	// docops.yaml must be loadable.
	cfg, err := config.Load(filepath.Join(root, "docops.yaml"))
	if err != nil {
		t.Fatalf("load scaffolded config: %v", err)
	}

	set, err := loader.Load(root, cfg)
	if err != nil {
		t.Fatalf("loader: %v", err)
	}

	report := validator.Validate(set, cfg)
	if !report.OK() {
		for _, f := range report.Errors {
			t.Errorf("validate error: %+v", f)
		}
		t.Fatalf("fresh init repo should validate clean (%d errors)", len(report.Errors))
	}

	// Schema files should be non-empty JSON.
	for _, name := range []string{"context.json", "adr.json", "task.json"} {
		body, err := os.ReadFile(filepath.Join(root, cfg.Paths.Schema, name))
		if err != nil {
			t.Errorf("schema %s: %v", name, err)
			continue
		}
		if len(body) < 32 {
			t.Errorf("schema %s suspiciously small: %d bytes", name, len(body))
		}
	}
}
