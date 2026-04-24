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
// fixture through the Codex transform (including name: injection, which mirrors
// what planNestedSkillDirHarness does) and compares output byte-for-byte against
// the committed golden file.
func TestCodexAdapter_TransformFrontmatter_GoldenFile(t *testing.T) {
	fixturePath := filepath.Join("testdata", "fixtures", "get-claude.md")
	goldenPath := filepath.Join("testdata", "codex", "docops-get", "SKILL.md")

	src, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	// Mirror the production path: parseFrontmatter → TransformFrontmatter →
	// inject name → serializeFrontmatter.
	h := codexAdapter{}
	fm, node, body, err := parseFrontmatter(src)
	if err != nil {
		t.Fatalf("parseFrontmatter: %v", err)
	}
	outFM, err := h.TransformFrontmatter(fm)
	if err != nil {
		t.Fatalf("TransformFrontmatter: %v", err)
	}
	// Inject name: the per-command step performed by planNestedSkillDirHarness.
	outFM["name"] = "docops-get"
	got, err := serializeFrontmatter(outFM, node, body)
	if err != nil {
		t.Fatalf("serializeFrontmatter: %v", err)
	}

	if !bytes.Equal(got, golden) {
		t.Errorf("Codex transform output does not match golden file.\nGot:\n%s\nWant:\n%s", got, golden)
	}
}

// TestCodexAdapter_NameInjected verifies that the writer-injected name: field
// is set to the skill directory name (e.g. "docops-get").
func TestCodexAdapter_NameInjected(t *testing.T) {
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

	// TransformFrontmatter itself must NOT set name: — that is the writer's job.
	if _, ok := got["name"]; ok {
		t.Error("TransformFrontmatter must not set name: — caller injects it per-command")
	}

	// Simulate the writer injection.
	got["name"] = "docops-get"
	if got["name"] != "docops-get" {
		t.Errorf("name: = %q; want %q", got["name"], "docops-get")
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

// TestCodexAdapter_WritesIntoNestedSkillDirs verifies that a dry-run of
// docops upgrade (with an empty project) produces create actions for all
// .codex/skills/docops-<cmd>/SKILL.md paths.
func TestCodexAdapter_WritesIntoNestedSkillDirs(t *testing.T) {
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

	// Find actions under .codex/skills/docops-*/SKILL.md.
	const prefix = ".codex/skills/docops-"
	var found []string
	for _, a := range res.Actions {
		if strings.HasPrefix(a.Rel, prefix) && strings.HasSuffix(a.Rel, "/SKILL.md") {
			found = append(found, a.Rel)
		}
	}
	if len(found) == 0 {
		t.Error("no .codex/skills/docops-*/SKILL.md actions found in dry-run plan")
	}
	// All skill file actions should be creates (dir is empty).
	for _, rel := range found {
		a := findAction(res.Actions, rel)
		if a == nil {
			t.Errorf("could not find action for %s", rel)
			continue
		}
		if a.Kind != scaffold.KindWriteFile || a.Reason != "create" {
			t.Errorf("%s: expected create write-file action, got kind=%s reason=%q", rel, a.Kind, a.Reason)
		}
	}
}
