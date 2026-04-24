package upgrader

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/logicwind/docops/internal/scaffold"
)

// TestOpenCodeAdapter_TransformFrontmatter_GoldenFile feeds the Claude-format
// fixture through the OpenCode transform and compares the output byte-for-byte
// against the committed golden file.
func TestOpenCodeAdapter_TransformFrontmatter_GoldenFile(t *testing.T) {
	fixturePath := filepath.Join("testdata", "fixtures", "get-claude.md")
	goldenPath := filepath.Join("testdata", "opencode", "docops-get.md")

	src, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	h := openCodeAdapter{}
	got, err := applyTransform(src, h.TransformFrontmatter)
	if err != nil {
		t.Fatalf("applyTransform: %v", err)
	}

	if !bytes.Equal(got, golden) {
		t.Errorf("OpenCode transform output does not match golden file.\nGot:\n%s\nWant:\n%s", got, golden)
	}
}

// TestOpenCodeAdapter_ToolMapping verifies the Claude→OpenCode tool-name mapping
// for the key cases: special renames, MCP passthrough, and lowercase fallback.
func TestOpenCodeAdapter_ToolMapping(t *testing.T) {
	cases := []struct {
		claudeTool string
		wantTool   string
	}{
		{"Read", "read"},
		{"Bash", "bash"},
		{"Write", "write"},
		{"Edit", "edit"},
		{"AskUserQuestion", "question"},
		{"SlashCommand", "skill"},
		{"TodoWrite", "todowrite"},
		{"WebFetch", "webfetch"},
		{"WebSearch", "websearch"},
		{"mcp__linear-server__list_issues", "mcp__linear-server__list_issues"},
		{"mcp__some-server__do_thing", "mcp__some-server__do_thing"},
	}
	for _, tc := range cases {
		got := convertOpenCodeToolName(tc.claudeTool)
		if got != tc.wantTool {
			t.Errorf("convertOpenCodeToolName(%q) = %q; want %q", tc.claudeTool, got, tc.wantTool)
		}
	}
}

// TestOpenCodeAdapter_AllowedToolsToToolsMap verifies that TransformFrontmatter
// converts an allowed-tools list to a tools map with true values and applies
// the tool-name mapping.
func TestOpenCodeAdapter_AllowedToolsToToolsMap(t *testing.T) {
	h := openCodeAdapter{}
	src := map[string]any{
		"name":          "get",
		"description":   "some command",
		"allowed-tools": []any{"Read", "Bash", "AskUserQuestion", "mcp__linear-server__list_issues"},
	}
	got, err := h.TransformFrontmatter(src)
	if err != nil {
		t.Fatalf("TransformFrontmatter error: %v", err)
	}

	// "name" must be dropped.
	if _, ok := got["name"]; ok {
		t.Error("name: key must be dropped in OpenCode transform")
	}
	// "allowed-tools" must be dropped.
	if _, ok := got["allowed-tools"]; ok {
		t.Error("allowed-tools: key must be converted and dropped")
	}
	// "tools" must be present as a map.
	toolsAny, ok := got["tools"]
	if !ok {
		t.Fatal("tools: key must be present in OpenCode transform output")
	}
	tools, ok := toolsAny.(map[string]bool)
	if !ok {
		t.Fatalf("tools: expected map[string]bool, got %T", toolsAny)
	}
	want := map[string]bool{
		"read":                             true,
		"bash":                             true,
		"question":                         true,
		"mcp__linear-server__list_issues":  true,
	}
	if !reflect.DeepEqual(tools, want) {
		// Sort for readable diff.
		gotKeys := make([]string, 0, len(tools))
		for k := range tools {
			gotKeys = append(gotKeys, k)
		}
		sort.Strings(gotKeys)
		wantKeys := make([]string, 0, len(want))
		for k := range want {
			wantKeys = append(wantKeys, k)
		}
		sort.Strings(wantKeys)
		t.Errorf("tools map mismatch:\n  got keys:  %v\n  want keys: %v", gotKeys, wantKeys)
	}
}

// TestOpenCodeAdapter_NameKeyDropped verifies that the "name:" key is
// unconditionally removed from the frontmatter (filename is the ID in OpenCode).
func TestOpenCodeAdapter_NameKeyDropped(t *testing.T) {
	h := openCodeAdapter{}
	src := map[string]any{
		"name":        "get",
		"description": "some command",
	}
	got, err := h.TransformFrontmatter(src)
	if err != nil {
		t.Fatalf("TransformFrontmatter error: %v", err)
	}
	if _, ok := got["name"]; ok {
		t.Error("name: key should be dropped by OpenCode TransformFrontmatter")
	}
	if _, ok := got["description"]; !ok {
		t.Error("description: key should be preserved")
	}
}

// TestOpenCodeAdapter_OtherKeysPreserved verifies that keys not explicitly
// transformed (description, argument-hint, custom keys) are passed through.
func TestOpenCodeAdapter_OtherKeysPreserved(t *testing.T) {
	h := openCodeAdapter{}
	src := map[string]any{
		"name":          "get",
		"description":   "some command",
		"argument-hint": "<ID>",
		"custom-key":    "value",
	}
	got, err := h.TransformFrontmatter(src)
	if err != nil {
		t.Fatalf("TransformFrontmatter error: %v", err)
	}
	for _, key := range []string{"description", "argument-hint", "custom-key"} {
		if _, ok := got[key]; !ok {
			t.Errorf("key %q should be preserved in OpenCode transform", key)
		}
	}
}

// TestOpenCodeAdapter_GlobalDir_Precedence tests that GlobalDir respects the
// four-level precedence: OPENCODE_CONFIG_DIR > dirname(OPENCODE_CONFIG) >
// XDG_CONFIG_HOME/opencode > ~/.config/opencode.
func TestOpenCodeAdapter_GlobalDir_Precedence(t *testing.T) {
	h := openCodeAdapter{}

	t.Run("OPENCODE_CONFIG_DIR", func(t *testing.T) {
		t.Setenv("OPENCODE_CONFIG_DIR", "/custom/opencode")
		t.Setenv("OPENCODE_CONFIG", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		dir, ok := h.GlobalDir()
		if !ok {
			t.Fatal("GlobalDir() ok=false; want true")
		}
		want := filepath.Join("/custom/opencode", "command")
		if dir != want {
			t.Errorf("GlobalDir() = %q; want %q", dir, want)
		}
	})

	t.Run("OPENCODE_CONFIG", func(t *testing.T) {
		t.Setenv("OPENCODE_CONFIG_DIR", "")
		t.Setenv("OPENCODE_CONFIG", "/home/user/.config/opencode/config.json")
		t.Setenv("XDG_CONFIG_HOME", "")
		dir, ok := h.GlobalDir()
		if !ok {
			t.Fatal("GlobalDir() ok=false; want true")
		}
		want := filepath.Join("/home/user/.config/opencode", "command")
		if dir != want {
			t.Errorf("GlobalDir() = %q; want %q", dir, want)
		}
	})

	t.Run("XDG_CONFIG_HOME", func(t *testing.T) {
		t.Setenv("OPENCODE_CONFIG_DIR", "")
		t.Setenv("OPENCODE_CONFIG", "")
		t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
		dir, ok := h.GlobalDir()
		if !ok {
			t.Fatal("GlobalDir() ok=false; want true")
		}
		want := filepath.Join("/xdg/config", "opencode", "command")
		if dir != want {
			t.Errorf("GlobalDir() = %q; want %q", dir, want)
		}
	})

	t.Run("fallback_home_config", func(t *testing.T) {
		t.Setenv("OPENCODE_CONFIG_DIR", "")
		t.Setenv("OPENCODE_CONFIG", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		dir, ok := h.GlobalDir()
		if !ok {
			t.Fatal("GlobalDir() ok=false; want true")
		}
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("UserHomeDir: %v", err)
		}
		want := filepath.Join(home, ".config", "opencode", "command")
		if dir != want {
			t.Errorf("GlobalDir() = %q; want %q", dir, want)
		}
	})
}

// TestOpenCodeAdapter_ByteIdentityViaUpgrader verifies that running a full
// planHarness cycle for the OpenCode adapter (with no existing dir) produces
// the expected docops-get.md output for a fixture that matches the shipped
// get.md format (no allowed-tools). This is an integration-level smoke test
// that the plumbing is wired end-to-end.
func TestOpenCodeAdapter_WritesIntoFlatDir(t *testing.T) {
	root := t.TempDir()

	// Write a minimal docops.yaml so Run() doesn't reject the directory.
	if err := os.WriteFile(filepath.Join(root, "docops.yaml"), []byte("project: test\n"), 0o644); err != nil {
		t.Fatalf("write docops.yaml: %v", err)
	}

	// Run the upgrader (dry-run); check that opencode actions are planned.
	res, err := Run(Options{Root: root, DryRun: true, Out: nil})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Find any action with rel starting with ".opencode/command/docops-".
	const prefix = ".opencode/command/docops-"
	var found []string
	for _, a := range res.Actions {
		if strings.HasPrefix(a.Rel, prefix) {
			found = append(found, a.Rel)
		}
	}
	if len(found) == 0 {
		t.Error("no .opencode/command/docops-*.md actions found in dry-run plan")
	}
	// All should be creates (dir is empty).
	for _, rel := range found {
		a := findAction(res.Actions, rel)
		if a == nil {
			t.Errorf("could not find action for %s", rel)
			continue
		}
		if a.Reason != "create" {
			t.Errorf("%s: expected create reason, got %q", rel, a.Reason)
		}
	}
}

// TestClaudeAdapter_ByteIdentityRegression asserts that running planHarness
// for the Claude adapter on a pre-seeded directory yields skip actions (not
// refresh actions) — proving that applyTransform with the identity transform
// produces byte-identical output to the shipped files.
func TestClaudeAdapter_ByteIdentityRegression(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "docops.yaml"), []byte("project: test\n"), 0o644); err != nil {
		t.Fatalf("write docops.yaml: %v", err)
	}

	// First run: apply all actions to populate the dirs.
	if _, err := Run(Options{Root: root, DryRun: false, Out: nopWriter{}}); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// Second run (dry): the Claude dir must report all skills as up-to-date.
	res, err := Run(Options{Root: root, DryRun: true, Out: nopWriter{}})
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}

	const prefix = ".claude/commands/docops/"
	for _, a := range res.Actions {
		if len(a.Rel) > len(prefix) && a.Rel[:len(prefix)] == prefix {
			if a.Kind != scaffold.KindSkip && a.Kind != scaffold.KindMkdir {
				t.Errorf("byte-identity regression: %s should be skip after round-trip, got %s (reason: %s)",
					a.Rel, a.Kind, a.Reason)
			}
		}
	}
}

// nopWriter discards all writes (implements io.Writer).
type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
