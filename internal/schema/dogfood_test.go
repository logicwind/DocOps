package schema_test

// This integration-style test runs the schema + validator against the
// live docs/ tree in this repository. Its job is to catch any drift
// between ADR-0002 and the hand-written dog-food documents. If this test
// fails, either the docs need fixing or ADR-0002 needs an update.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/schema"
)

func TestDogfoodDocsValidate(t *testing.T) {
	repoRoot := findRepoRoot(t)
	cfg, _, err := config.FindAndLoad(repoRoot)
	if err != nil {
		t.Fatalf("load docops.yaml: %v", err)
	}
	schemaCfg := schema.Config{ContextTypes: cfg.ContextTypes}

	walk := func(dir string) []string {
		entries, err := os.ReadDir(filepath.Join(repoRoot, dir))
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		var files []string
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			files = append(files, filepath.Join(dir, e.Name()))
		}
		return files
	}

	var failures int
	for _, rel := range walk(cfg.Paths.Context) {
		if err := validateFile(t, repoRoot, rel, schemaCfg); err != nil {
			t.Errorf("%s: %v", rel, err)
			failures++
		}
	}
	for _, rel := range walk(cfg.Paths.Decisions) {
		if err := validateFile(t, repoRoot, rel, schemaCfg); err != nil {
			t.Errorf("%s: %v", rel, err)
			failures++
		}
	}
	for _, rel := range walk(cfg.Paths.Tasks) {
		if err := validateFile(t, repoRoot, rel, schemaCfg); err != nil {
			t.Errorf("%s: %v", rel, err)
			failures++
		}
	}
	if failures > 0 {
		t.Fatalf("%d dog-food document(s) failed validation", failures)
	}
}

func validateFile(t *testing.T, root, rel string, cfg schema.Config) error {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return err
	}
	fm, _, err := schema.SplitFrontmatter(raw)
	if err != nil {
		return err
	}
	kind, ok := schema.KindFromFilename(filepath.Base(rel))
	if !ok {
		return nil // Non-DocOps markdown (e.g. STATE.md) — skip.
	}
	switch kind {
	case schema.KindContext:
		c, err := schema.ParseContext(fm)
		if err != nil {
			return err
		}
		return schema.ValidateContext(c, cfg)
	case schema.KindADR:
		a, err := schema.ParseADR(fm)
		if err != nil {
			return err
		}
		return schema.ValidateADR(a)
	case schema.KindTask:
		tk, err := schema.ParseTask(fm)
		if err != nil {
			return err
		}
		return schema.ValidateTask(tk)
	}
	return nil
}

// findRepoRoot walks up from the test's working directory looking for
// docops.yaml. Tests run with the package dir as CWD, so the config is
// always several parents up.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "docops.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no docops.yaml found above %s", wd)
		}
		dir = parent
	}
}
