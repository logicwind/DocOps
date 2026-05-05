package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDetectBrownfield_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	got := DetectBrownfield(dir)
	if got.Brownfield {
		t.Fatalf("empty dir flagged brownfield with signals %v", got.Signals)
	}
}

func TestDetectBrownfield_PackageJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), "{}")
	got := DetectBrownfield(dir)
	if !got.Brownfield {
		t.Fatalf("dir with package.json not flagged brownfield")
	}
	if !contains(got.Signals, "package.json") {
		t.Errorf("signals %v missing package.json", got.Signals)
	}
}

func TestDetectBrownfield_GoMod(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module foo\n")
	got := DetectBrownfield(dir)
	if !got.Brownfield {
		t.Fatalf("dir with go.mod not flagged brownfield")
	}
}

func TestDetectBrownfield_SrcDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "src", "main.ts"), "export {}")
	got := DetectBrownfield(dir)
	if !got.Brownfield {
		t.Fatalf("dir with src/ not flagged brownfield")
	}
	if !contains(got.Signals, "src/") {
		t.Errorf("signals %v missing src/", got.Signals)
	}
}

func TestDetectBrownfield_EmptySrcDirIgnored(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := DetectBrownfield(dir)
	if got.Brownfield {
		t.Fatalf("empty src/ should not flag brownfield: %v", got.Signals)
	}
}

func TestDetectBrownfield_GitCommits(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)

	// Make 11 commits — above the threshold of 10.
	for i := 0; i < 11; i++ {
		writeFile(t, filepath.Join(dir, "README.md"), "v"+string(rune('a'+i)))
		runIn(t, dir, "git", "add", "README.md")
		runIn(t, dir, "git", "commit", "-m", "c"+string(rune('a'+i)))
	}

	got := DetectBrownfield(dir)
	if !got.Brownfield {
		t.Fatalf("repo with 11 commits not flagged brownfield: %v", got.Signals)
	}
	var hasCommits bool
	for _, s := range got.Signals {
		if len(s) > 8 && s[len(s)-7:] == "commits" {
			hasCommits = true
		}
	}
	if !hasCommits {
		t.Errorf("signals %v missing 'N commits'", got.Signals)
	}
}

func TestDetectBrownfield_FewCommitsIgnored(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	writeFile(t, filepath.Join(dir, "x"), "y")
	runIn(t, dir, "git", "add", "x")
	runIn(t, dir, "git", "commit", "-m", "c")
	got := DetectBrownfield(dir)
	if got.Brownfield {
		t.Fatalf("repo with 1 commit and no other signals should be greenfield: %v", got.Signals)
	}
}

// helpers --------------------------------------------------------

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	runIn(t, dir, "git", "init", "-q")
	runIn(t, dir, "git", "config", "user.email", "test@example.com")
	runIn(t, dir, "git", "config", "user.name", "test")
	runIn(t, dir, "git", "config", "commit.gpgsign", "false")
}

func runIn(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
