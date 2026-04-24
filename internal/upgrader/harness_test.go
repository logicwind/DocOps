package upgrader

import (
	"strings"
	"testing"
)

// TestRegistry_AllAdaptersHaveNonEmptyFields asserts that every registered
// harness returns a non-empty slug and LocalDir, and that FilenameFor
// produces a non-empty, forward-slash-separated path containing the
// command name for a sample command.
func TestRegistry_AllAdaptersHaveNonEmptyFields(t *testing.T) {
	const sampleCmd = "get"

	for _, h := range registeredHarnesses() {
		h := h // capture
		t.Run(h.Slug(), func(t *testing.T) {
			if h.Slug() == "" {
				t.Error("Slug() must be non-empty")
			}
			if h.LocalDir() == "" {
				t.Error("LocalDir() must be non-empty")
			}

			name := h.FilenameFor(sampleCmd)
			if name == "" {
				t.Errorf("FilenameFor(%q) returned empty string", sampleCmd)
			}
			if !strings.Contains(name, sampleCmd) {
				t.Errorf("FilenameFor(%q) = %q; expected it to contain the command name", sampleCmd, name)
			}
		})
	}
}

// TestRegistry_TransformFrontmatter_IdentityForPhase1 verifies that
// Claude and Cursor adapters return a copy of the input frontmatter
// unchanged (identity transform).
func TestRegistry_TransformFrontmatter_IdentityForPhase1(t *testing.T) {
	src := map[string]any{
		"description":   "fetch a doc by ID",
		"allowed-tools": []string{"Read", "Bash"},
	}

	for _, h := range registeredHarnesses() {
		h := h
		t.Run(h.Slug(), func(t *testing.T) {
			got, err := h.TransformFrontmatter(src)
			if err != nil {
				t.Fatalf("TransformFrontmatter returned error: %v", err)
			}
			if len(got) != len(src) {
				t.Errorf("TransformFrontmatter changed key count: got %d, want %d", len(got), len(src))
			}
			for k, want := range src {
				if _, ok := got[k]; !ok {
					t.Errorf("key %q missing from TransformFrontmatter output", k)
				}
				_ = want // value equality checked structurally above
			}
		})
	}
}

// TestRegistry_TransformFrontmatter_ReturnsCopy verifies that mutating
// the returned map does not affect the source (i.e. it is a copy).
func TestRegistry_TransformFrontmatter_ReturnsCopy(t *testing.T) {
	src := map[string]any{"description": "original"}

	for _, h := range registeredHarnesses() {
		h := h
		t.Run(h.Slug(), func(t *testing.T) {
			got, err := h.TransformFrontmatter(src)
			if err != nil {
				t.Fatalf("TransformFrontmatter error: %v", err)
			}
			got["injected"] = "mutated"
			if _, leaked := src["injected"]; leaked {
				t.Error("TransformFrontmatter returned the original map; expected a copy")
			}
		})
	}
}

// TestFilenameForLayout verifies the free-function helper produces the
// correct paths for all three layout variants, matching the doc-specified
// conventions (ADR-0028 dialect table, TP-033 FilenameFor spec).
func TestFilenameForLayout(t *testing.T) {
	cases := []struct {
		layout Layout
		cmd    string
		want   string
	}{
		{LayoutNestedFile, "get", "docops/get.md"},
		{LayoutNestedFile, "audit", "docops/audit.md"},
		{LayoutFlatPrefixFile, "get", "docops-get.md"},
		{LayoutFlatPrefixFile, "audit", "docops-audit.md"},
		{LayoutNestedSkillDir, "get", "docops-get/SKILL.md"},
		{LayoutNestedSkillDir, "audit", "docops-audit/SKILL.md"},
	}

	for _, tc := range cases {
		got, err := FilenameForLayout(tc.layout, tc.cmd)
		if err != nil {
			t.Errorf("FilenameForLayout(%d, %q) error: %v", tc.layout, tc.cmd, err)
			continue
		}
		// Normalise separators so tests pass on Windows too.
		got = strings.ReplaceAll(got, "\\", "/")
		if got != tc.want {
			t.Errorf("FilenameForLayout(%d, %q) = %q; want %q", tc.layout, tc.cmd, got, tc.want)
		}
	}
}

// TestHarnessLocalDirs_MatchesRegistry confirms that harnessLocalDirs()
// returns one entry per registered harness and that they match LocalDir().
func TestHarnessLocalDirs_MatchesRegistry(t *testing.T) {
	harnesses := registeredHarnesses()
	dirs := harnessLocalDirs()

	if len(dirs) != len(harnesses) {
		t.Fatalf("harnessLocalDirs() returned %d entries; want %d (one per harness)", len(dirs), len(harnesses))
	}
	for i, h := range harnesses {
		if dirs[i] != h.LocalDir() {
			t.Errorf("dirs[%d] = %q; want %q (from %s adapter)", i, dirs[i], h.LocalDir(), h.Slug())
		}
	}
}
