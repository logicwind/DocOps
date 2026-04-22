package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/schema"
)

func TestLoad_DiscoversAllKinds(t *testing.T) {
	root := t.TempDir()
	writeDocs(t, root, map[string]string{
		"docs/context/CTX-001-foo.md":    ctxDoc("hi", "memo"),
		"docs/decisions/ADR-0001-bar.md": adrDoc("hi"),
		"docs/tasks/TP-001-baz.md":       taskDoc("hi", []string{"ADR-0001"}),
	})
	cfg := config.Default()
	set, err := Load(root, cfg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, id := range []string{"CTX-001", "ADR-0001", "TP-001"} {
		if !set.Has(id) {
			t.Errorf("missing %s in DocSet", id)
		}
	}
	if got := set.Get("TP-001").Kind; got != schema.KindTask {
		t.Errorf("kind = %q; want %q", got, schema.KindTask)
	}
}

func TestLoad_RejectsDuplicateID(t *testing.T) {
	root := t.TempDir()
	writeDocs(t, root, map[string]string{
		"docs/decisions/ADR-0001-a.md": adrDoc("a"),
		"docs/decisions/ADR-0001-b.md": adrDoc("b"),
	})
	if _, err := Load(root, config.Default()); err == nil {
		t.Fatalf("expected duplicate-ID error")
	}
}

func TestLoad_TolerantOfMissingDirs(t *testing.T) {
	root := t.TempDir()
	// Only a context dir exists; decisions/ and tasks/ are absent.
	writeDocs(t, root, map[string]string{
		"docs/context/CTX-001-foo.md": ctxDoc("hi", "memo"),
	})
	set, err := Load(root, config.Default())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(set.Docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(set.Docs))
	}
}

func TestLoad_RetainsDocOnParseError(t *testing.T) {
	// Broken YAML: unterminated frontmatter. Loader should still produce a
	// Doc record with ParseErr set so the validator can report it.
	root := t.TempDir()
	path := filepath.Join(root, "docs/decisions/ADR-0001-broken.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\ntitle: x\nno closing fence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	set, err := Load(root, config.Default())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	d := set.Get("ADR-0001")
	if d == nil {
		t.Fatal("missing ADR-0001")
	}
	if d.ParseErr == nil {
		t.Errorf("expected ParseErr on broken frontmatter")
	}
}

func TestIDFromFilename(t *testing.T) {
	cases := map[string]string{
		"ADR-0012-foo-bar.md":    "ADR-0012",
		"TP-003-implement-x.md":  "TP-003",
		"CTX-001-vision.md":      "CTX-001",
		"CTX-999.md":             "CTX-999",
	}
	for name, want := range cases {
		kind, _ := schema.KindFromFilename(name)
		if got := idFromFilename(name, kind); got != want {
			t.Errorf("idFromFilename(%q) = %q; want %q", name, got, want)
		}
	}
}

// ---- helpers ----

func writeDocs(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func ctxDoc(title, typ string) string {
	return "---\ntitle: " + title + "\ntype: " + typ + "\n---\n\nbody\n"
}

func adrDoc(title string) string {
	return "---\ntitle: " + title + "\nstatus: accepted\ncoverage: required\ndate: 2026-04-22\n---\n\nbody\n"
}

func taskDoc(title string, requires []string) string {
	r := "["
	for i, v := range requires {
		if i > 0 {
			r += ", "
		}
		r += v
	}
	r += "]"
	return "---\ntitle: " + title + "\nstatus: backlog\nrequires: " + r + "\n---\n\nbody\n"
}
