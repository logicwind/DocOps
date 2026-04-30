package upgrader

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/logicwind/docops/internal/scaffold"
)

// TestCodexAdapter_TransformFrontmatter_GoldenFile feeds the Claude-format
// fixture through the Codex transform (used for per-subroutine files
// inside the docops/ skill bundle) and compares the output byte-for-byte
// against the committed golden file.
func TestCodexAdapter_TransformFrontmatter_GoldenFile(t *testing.T) {
	fixturePath := filepath.Join("testdata", "fixtures", "get-claude.md")
	goldenPath := filepath.Join("testdata", "codex", "docops", "get.md")

	src, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	got, err := applyTransform(src, codexAdapter{}.TransformFrontmatter)
	if err != nil {
		t.Fatalf("applyTransform: %v", err)
	}

	if !bytes.Equal(got, golden) {
		t.Errorf("Codex transform output does not match golden file.\nGot:\n%s\nWant:\n%s", got, golden)
	}
}

// TestCodexAdapter_NameDropped verifies TransformFrontmatter drops the
// per-command name: field. The bundle as a whole has a single name set in
// SKILL.md; subroutine files do not need their own.
func TestCodexAdapter_NameDropped(t *testing.T) {
	h := codexAdapter{}
	src := map[string]any{
		"name":          "get",
		"description":   "some command",
		"allowed-tools": []any{"Read", "Bash"},
	}
	got, err := h.TransformFrontmatter(src)
	if err != nil {
		t.Fatalf("TransformFrontmatter error: %v", err)
	}
	if _, ok := got["name"]; ok {
		t.Error("name: must be dropped on per-subroutine files; only the bundle's SKILL.md carries name:")
	}
}

// TestCodexAdapter_AllowedToolsDropped verifies that allowed-tools: is not
// present in the Codex output (Codex skills do not use this field).
func TestCodexAdapter_AllowedToolsDropped(t *testing.T) {
	h := codexAdapter{}
	src := map[string]any{
		"name":          "get",
		"description":   "some command",
		"allowed-tools": []any{"Read", "Bash", "AskUserQuestion"},
	}
	got, err := h.TransformFrontmatter(src)
	if err != nil {
		t.Fatalf("TransformFrontmatter error: %v", err)
	}
	if _, ok := got["allowed-tools"]; ok {
		t.Error("allowed-tools: must be dropped in Codex transform")
	}
}

// TestCodexAdapter_DescriptionPreserved verifies that description: survives
// the Codex transform unchanged.
func TestCodexAdapter_DescriptionPreserved(t *testing.T) {
	h := codexAdapter{}
	src := map[string]any{
		"name":          "get",
		"description":   "Look up a doc by ID.",
		"allowed-tools": []any{"Read"},
	}
	got, err := h.TransformFrontmatter(src)
	if err != nil {
		t.Fatalf("TransformFrontmatter error: %v", err)
	}
	if got["description"] != "Look up a doc by ID." {
		t.Errorf("description: = %q; want %q", got["description"], "Look up a doc by ID.")
	}
}

// TestCodexAdapter_GlobalDir_Precedence tests that GlobalDir respects the
// two-level precedence: CODEX_HOME/skills > ~/.codex/skills.
func TestCodexAdapter_GlobalDir_Precedence(t *testing.T) {
	h := codexAdapter{}

	t.Run("CODEX_HOME_set", func(t *testing.T) {
		t.Setenv("CODEX_HOME", "/custom/codex")
		dir, ok := h.GlobalDir()
		if !ok {
			t.Fatal("GlobalDir() ok=false; want true")
		}
		want := filepath.Join("/custom/codex", "skills")
		if dir != want {
			t.Errorf("GlobalDir() = %q; want %q", dir, want)
		}
	})

	t.Run("CODEX_HOME_unset_uses_home_dot_codex", func(t *testing.T) {
		t.Setenv("CODEX_HOME", "")
		dir, ok := h.GlobalDir()
		if !ok {
			t.Fatal("GlobalDir() ok=false; want true")
		}
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("UserHomeDir: %v", err)
		}
		want := filepath.Join(home, ".codex", "skills")
		if dir != want {
			t.Errorf("GlobalDir() = %q; want %q", dir, want)
		}
	})
}

// TestCodexAdapter_WritesSkillBundle verifies that a dry-run of
// docops upgrade on an empty project plans (a) one SKILL.md write at
// the bundle root and (b) one cookbook/<cmd>.md write per shipped
// subroutine (per ADR-0031).
func TestCodexAdapter_WritesSkillBundle(t *testing.T) {
	root := t.TempDir()

	// Write a minimal docops.yaml so Run() doesn't reject the directory.
	if err := os.WriteFile(filepath.Join(root, "docops.yaml"), []byte("project: test\n"), 0o644); err != nil {
		t.Fatalf("write docops.yaml: %v", err)
	}

	// Dry-run.
	res, err := Run(Options{Root: root, DryRun: true, Out: nil})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Find file actions under .codex/skills/docops/.
	const prefix = ".codex/skills/docops/"
	const cookbookPrefix = "cookbook/"
	var skill string
	var subroutines []string
	for _, a := range res.Actions {
		if a.Kind != scaffold.KindWriteFile {
			continue
		}
		if !strings.HasPrefix(a.Rel, prefix) {
			continue
		}
		rel := strings.TrimPrefix(a.Rel, prefix)
		if rel == "SKILL.md" {
			skill = a.Rel
			continue
		}
		if !strings.HasPrefix(rel, cookbookPrefix) {
			t.Errorf("unexpected path inside bundle (expected SKILL.md or cookbook/*): %s", a.Rel)
			continue
		}
		base := strings.TrimPrefix(rel, cookbookPrefix)
		if strings.Contains(base, "/") {
			t.Errorf("unexpected deep-nested path inside cookbook: %s", a.Rel)
			continue
		}
		subroutines = append(subroutines, base)
	}

	if skill == "" {
		t.Error("missing .codex/skills/docops/SKILL.md write action")
	}
	if len(subroutines) == 0 {
		t.Error("no per-subroutine writes found in .codex/skills/docops/cookbook/")
	}
	for _, name := range subroutines {
		if !strings.HasSuffix(name, ".md") {
			t.Errorf("non-.md file in cookbook: %s", name)
		}
		if name == "SKILL.md" {
			t.Errorf("SKILL.md leaked into cookbook list")
		}
	}
}
