package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault_HasSaneValues(t *testing.T) {
	c := Default()
	if c.Version != 1 {
		t.Errorf("version = %d; want 1", c.Version)
	}
	if c.Paths.Context == "" || c.Paths.Decisions == "" || c.Paths.Tasks == "" {
		t.Errorf("expected non-empty paths: %+v", c.Paths)
	}
	if len(c.ContextTypes) == 0 {
		t.Errorf("expected non-empty context_types")
	}
}

func TestApplyDefaults_FillsMissingFields(t *testing.T) {
	c := Config{Paths: Paths{Context: "custom/ctx"}, ContextTypes: []string{"memo"}}
	c.ApplyDefaults()
	if c.Paths.Context != "custom/ctx" {
		t.Errorf("Paths.Context overwritten: got %q", c.Paths.Context)
	}
	if c.Paths.Decisions != "docs/decisions" {
		t.Errorf("Paths.Decisions default not applied: got %q", c.Paths.Decisions)
	}
	if len(c.ContextTypes) != 1 || c.ContextTypes[0] != "memo" {
		t.Errorf("ContextTypes overwritten: got %v", c.ContextTypes)
	}
}

func TestLoad_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docops.yaml")
	yaml := "version: 1\npaths:\n  context: cx\ncontext_types:\n  - memo\n  - prd\n"
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Paths.Context != "cx" {
		t.Errorf("Paths.Context = %q; want cx", cfg.Paths.Context)
	}
	if cfg.Paths.Decisions != "docs/decisions" {
		t.Errorf("Paths.Decisions default lost: %q", cfg.Paths.Decisions)
	}
	if len(cfg.ContextTypes) != 2 {
		t.Errorf("ContextTypes = %v", cfg.ContextTypes)
	}
}

func TestFindAndLoad_WalksUp(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "repo")
	nested := filepath.Join(root, "sub", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docops.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, found, err := FindAndLoad(nested)
	if err != nil {
		t.Fatalf("FindAndLoad: %v", err)
	}
	if found != root {
		t.Errorf("found = %q; want %q", found, root)
	}
	if cfg.Version != 1 {
		t.Errorf("version = %d", cfg.Version)
	}
}
