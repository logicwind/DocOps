package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// brownfieldFiles is the set of well-known top-level files whose
// presence flags a repo as brownfield. Order is the order signals
// will be reported back to the user, so favor "loud" framework
// markers first.
var brownfieldFiles = []string{
	"package.json",
	"go.mod",
	"Cargo.toml",
	"pyproject.toml",
	"Gemfile",
	"pom.xml",
	"composer.json",
	"requirements.txt",
}

// brownfieldDirs is the set of well-known top-level source directories
// that flag a repo as brownfield when present and non-empty.
var brownfieldDirs = []string{
	"src",
	"app",
	"lib",
}

// commitThreshold is the git-history depth above which a repo is
// treated as brownfield even with no other markers.
const commitThreshold = 10

// DetectionResult captures the brownfield/greenfield decision plus the
// human-readable signals that drove it. Used by `docops init` to
// route its closing next-step block.
type DetectionResult struct {
	Brownfield bool
	// Signals lists the matched markers, e.g. ["go.mod", "src/", "184 commits"].
	// Always populated for brownfield; empty for greenfield.
	Signals []string
}

// DetectBrownfield inspects root with cheap file + git checks and
// returns whether the repo looks like an existing codebase. Detection
// is heuristic, not authoritative: it errs toward brownfield when
// signals are present and toward greenfield when nothing matches.
//
// No network calls; no recursive directory scans. Targets <50ms on
// any reasonable repo.
func DetectBrownfield(root string) DetectionResult {
	res := DetectionResult{}

	for _, name := range brownfieldFiles {
		if fileExists(filepath.Join(root, name)) {
			res.Signals = append(res.Signals, name)
		}
	}
	for _, name := range brownfieldDirs {
		if dirNonEmpty(filepath.Join(root, name)) {
			res.Signals = append(res.Signals, name+"/")
		}
	}
	if n, ok := commitCount(root); ok && n > commitThreshold {
		res.Signals = append(res.Signals, strconv.Itoa(n)+" commits")
	}

	res.Brownfield = len(res.Signals) > 0
	return res
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirNonEmpty(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// commitCount returns the count of commits reachable from HEAD, or
// (0, false) if the repo is not a git checkout or git is unavailable.
func commitCount(root string) (int, bool) {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return 0, false
	}
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, false
	}
	return n, true
}
