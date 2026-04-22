package main

import (
	"os"
	"path/filepath"
	"testing"
)

// withGitDir creates a minimal .git/hooks directory so planHook does not
// short-circuit to a skip when the test needs a real init run.
func withGitDir(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatalf("mkdir .git/hooks: %v", err)
	}
}

// chdirTo changes the working directory to dir for the duration of the test.
func chdirTo(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// TestCmdInit_NoDirArgUsessCwd verifies that omitting [dir] inits into cwd.
func TestCmdInit_NoDirArgUsesCwd(t *testing.T) {
	root := t.TempDir()
	withGitDir(t, root)
	chdirTo(t, root)

	// --yes skips the TTY prompt; stdin is not a terminal in tests.
	code := cmdInit([]string{"--yes"})
	if code != 0 {
		t.Fatalf("cmdInit --yes returned %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(root, "docops.yaml")); err != nil {
		t.Errorf("docops.yaml missing after init in cwd: %v", err)
	}
}

// TestCmdInit_PositionalDirCreatesAndInits verifies that a missing [dir] is
// created and scaffolded.
func TestCmdInit_PositionalDirCreatesAndInits(t *testing.T) {
	root := t.TempDir()
	withGitDir(t, root)
	chdirTo(t, root)

	newDir := filepath.Join(root, "subproject")
	// newDir does not exist yet.
	if _, err := os.Stat(newDir); !os.IsNotExist(err) {
		t.Fatalf("expected newDir to be absent before init")
	}

	code := cmdInit([]string{"subproject", "--yes"})
	if code != 0 {
		t.Fatalf("cmdInit returned %d, want 0", code)
	}

	if _, err := os.Stat(newDir); err != nil {
		t.Errorf("subproject directory not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newDir, "docops.yaml")); err != nil {
		t.Errorf("docops.yaml missing inside subproject: %v", err)
	}
}

// TestCmdInit_PositionalDirExistingDirOK verifies that an existing directory
// is accepted without error.
func TestCmdInit_PositionalDirExistingDirOK(t *testing.T) {
	root := t.TempDir()
	withGitDir(t, root)
	chdirTo(t, root)

	sub := filepath.Join(root, "existing")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	code := cmdInit([]string{"existing", "--yes"})
	if code != 0 {
		t.Fatalf("cmdInit into existing dir returned %d, want 0", code)
	}
}

// TestCmdInit_PositionalFileRejectsWithExitTwo verifies that passing a
// regular file as [dir] exits with code 2.
func TestCmdInit_PositionalFileRejectsWithExitTwo(t *testing.T) {
	root := t.TempDir()
	chdirTo(t, root)

	// Create a regular file at the target path.
	filePath := filepath.Join(root, "notadir")
	if err := os.WriteFile(filePath, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	code := cmdInit([]string{"notadir", "--yes"})
	if code != 2 {
		t.Errorf("expected exit 2 when positional is a file, got %d", code)
	}
}

// TestCmdInit_DryRunWritesNothing verifies --dry-run leaves the fs unchanged.
func TestCmdInit_DryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	withGitDir(t, root)
	chdirTo(t, root)

	code := cmdInit([]string{"--dry-run"})
	if code != 0 {
		t.Fatalf("--dry-run returned %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(root, "docops.yaml")); !os.IsNotExist(err) {
		t.Errorf("--dry-run must not create docops.yaml")
	}
}

// TestCmdInit_YesFlagSkipsPrompt verifies --yes allows non-interactive runs.
// (The test environment has no real TTY, so this also covers the non-TTY path.)
func TestCmdInit_YesFlagSkipsPrompt(t *testing.T) {
	root := t.TempDir()
	withGitDir(t, root)
	chdirTo(t, root)

	code := cmdInit([]string{"--yes"})
	if code != 0 {
		t.Fatalf("--yes returned %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(root, "docops.yaml")); err != nil {
		t.Errorf("docops.yaml missing after --yes init: %v", err)
	}
}

// TestSplitInitArgs verifies the positional/flag separation helper.
func TestSplitInitArgs(t *testing.T) {
	cases := []struct {
		in      []string
		wantPos string
		wantLen int // length of flagArgs
	}{
		{[]string{"mydir", "--yes"}, "mydir", 1},
		{[]string{"--yes", "--dry-run"}, "", 2},
		{[]string{}, "", 0},
		{[]string{"mydir"}, "mydir", 0},
		{[]string{"--force", "mydir"}, "mydir", 1},
	}
	for _, tc := range cases {
		pos, flags := splitInitArgs(tc.in)
		if pos != tc.wantPos {
			t.Errorf("splitInitArgs(%v) positional = %q, want %q", tc.in, pos, tc.wantPos)
		}
		if len(flags) != tc.wantLen {
			t.Errorf("splitInitArgs(%v) len(flagArgs) = %d, want %d", tc.in, len(flags), tc.wantLen)
		}
	}
}
