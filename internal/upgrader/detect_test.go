package upgrader

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestResolveHarnesses_NilReturnsAll(t *testing.T) {
	got := resolveHarnesses(nil)
	if len(got) != len(registry) {
		t.Fatalf("nil should return full registry (%d), got %d", len(registry), len(got))
	}
	for i, h := range got {
		if h.Slug() != registry[i].Slug() {
			t.Errorf("ordering mismatch at %d: got %q, want %q", i, h.Slug(), registry[i].Slug())
		}
	}
}

func TestResolveHarnesses_ExplicitFilter(t *testing.T) {
	got := resolveHarnesses([]string{"claude", "codex"})
	if len(got) != 2 {
		t.Fatalf("expected 2 harnesses, got %d", len(got))
	}
	slugs := []string{got[0].Slug(), got[1].Slug()}
	sort.Strings(slugs)
	want := []string{"claude", "codex"}
	for i := range want {
		if slugs[i] != want[i] {
			t.Errorf("slug[%d]: got %q, want %q", i, slugs[i], want[i])
		}
	}
}

func TestResolveHarnesses_EmptySliceMeansNone(t *testing.T) {
	got := resolveHarnesses([]string{})
	if len(got) != 0 {
		t.Fatalf("empty non-nil slice should return 0 harnesses, got %d", len(got))
	}
}

func TestResolveHarnesses_UnknownSlugIgnored(t *testing.T) {
	got := resolveHarnesses([]string{"claude", "windsurf-not-registered"})
	if len(got) != 1 {
		t.Fatalf("unknown slug should be ignored, got %d harnesses", len(got))
	}
	if got[0].Slug() != "claude" {
		t.Errorf("got %q, want claude", got[0].Slug())
	}
}

func TestKnownHarnessSlugs_RegistryOrder(t *testing.T) {
	got := KnownHarnessSlugs()
	want := []string{"claude", "cursor", "opencode", "codex"}
	if len(got) != len(want) {
		t.Fatalf("want %d slugs, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("slug[%d]: got %q, want %q (registry order must be stable)", i, got[i], want[i])
		}
	}
}

// TestDetectInstalledHarnesses_LocalDirWins confirms that a project-local
// harness dir is sufficient signal even when globals are absent.
func TestDetectInstalledHarnesses_LocalDirWins(t *testing.T) {
	root := t.TempDir()

	// Isolate globals: point HOME and CODEX_HOME at empty dirs.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("OPENCODE_CONFIG", "")

	// Create .claude/commands so claude detects via LocalDir.
	if err := os.MkdirAll(filepath.Join(root, ".claude", "commands"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got := DetectInstalledHarnesses(root)

	// Claude must be present (local dir exists).
	// Cursor/OpenCode/Codex should be absent (no local dir, no global dir).
	if !containsSlug(got, "claude") {
		t.Errorf("expected claude to be detected (local dir present), got %v", got)
	}
	for _, absent := range []string{"cursor", "opencode", "codex"} {
		if containsSlug(got, absent) {
			t.Errorf("did not expect %q to be detected (no local dir, isolated globals), got %v", absent, got)
		}
	}
}

// TestDetectInstalledHarnesses_EmptyRootWithNoHome returns nothing when
// there are no install signals anywhere.
func TestDetectInstalledHarnesses_EmptyRootWithNoHome(t *testing.T) {
	root := t.TempDir()
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("OPENCODE_CONFIG", "")

	got := DetectInstalledHarnesses(root)
	if len(got) != 0 {
		t.Errorf("expected no harnesses on fresh dirs with isolated globals, got %v", got)
	}
}

// TestDetectInstalledHarnesses_CodexHomeEnv verifies the CODEX_HOME env
// var drives detection even when $HOME is empty.
func TestDetectInstalledHarnesses_CodexHomeEnv(t *testing.T) {
	root := t.TempDir()
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("OPENCODE_CONFIG", "")

	// Populate a Codex skills dir via CODEX_HOME.
	codexHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(codexHome, "skills"), 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("CODEX_HOME", codexHome)

	got := DetectInstalledHarnesses(root)
	if !containsSlug(got, "codex") {
		t.Errorf("expected codex to detect via CODEX_HOME, got %v", got)
	}
}

func containsSlug(slugs []string, want string) bool {
	for _, s := range slugs {
		if s == want {
			return true
		}
	}
	return false
}
