package state

import (
	"os/exec"
	"strings"
)

// RecentGitActivity runs `git log` over the doc paths and returns up to
// maxEntries commits that touched those paths since the given date string
// (YYYY-MM-DD).  Returns nil (not an error) when git is unavailable — the
// caller falls back to index last_touched values.
//
// Output format requested from git: ISO timestamp TAB subject TAB short sha.
// We use --pretty=format:"%cI%x09%s%x09%H" and trim the hash to 7 chars.
func RecentGitActivity(root, since string, docPaths []string, maxEntries int) ([]ActivityEntry, error) {
	args := []string{
		"log",
		"--since=" + since,
		"--pretty=format:%cI\t%s\t%H",
		"--",
	}
	args = append(args, docPaths...)

	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		// git unavailable or not a git repo — caller uses fallback.
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var entries []ActivityEntry
	seen := map[string]struct{}{} // deduplicate by sha+subject
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		ts, subject, sha := parts[0], parts[1], parts[2]
		date := ""
		if len(ts) >= 10 {
			date = ts[:10]
		}
		shortSHA := sha
		if len(sha) > 7 {
			shortSHA = sha[:7]
		}
		key := shortSHA + "\x00" + subject
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		entries = append(entries, ActivityEntry{
			Date:     date,
			ShortSHA: shortSHA,
			Subject:  subject,
			Source:   "git",
		})
		if maxEntries > 0 && len(entries) >= maxEntries {
			break
		}
	}
	return entries, nil
}
