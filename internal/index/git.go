package index

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// gitLastTouched returns the commit timestamp for the file at relPath within
// root using `git log -1 --format=%cI`. If git is unavailable or the file is
// untracked, it falls back to the filesystem mtime.
//
// The second return value is "git" or "mtime" for diagnostics.
func gitLastTouched(root, relPath string) (time.Time, string) {
	t, src, err := tryGit(root, relPath)
	if err != nil {
		log.Printf("index: git log for %s: %v — falling back to mtime", relPath, err)
		return mtime(filepath.Join(root, relPath)), "mtime"
	}
	if t.IsZero() {
		// File exists but has never been committed (untracked / new).
		return mtime(filepath.Join(root, relPath)), "mtime"
	}
	return t, src
}

// tryGit attempts to run git log and parse its output.
func tryGit(root, relPath string) (time.Time, string, error) {
	cmd := exec.Command("git", "log", "-1", "--format=%cI", "--", relPath)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return time.Time{}, "", err
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return time.Time{}, "git", nil // untracked
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// git can emit offsets like +05:30 which time.RFC3339 handles; try
		// the numeric tz form just in case.
		return time.Time{}, "", err
	}
	return t, "git", nil
}

// mtime returns the file's modification time, or time.Now() if stat fails.
func mtime(absPath string) time.Time {
	info, err := os.Stat(absPath)
	if err != nil {
		return time.Now()
	}
	return info.ModTime()
}
