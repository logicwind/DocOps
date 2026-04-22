package upgrader

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ManifestFilename is the dot-prefixed sentinel docops writes inside
// each owned skill directory to record what it scaffolded. On
// subsequent upgrades, the manifest scopes deletion to "files we
// previously wrote" so user-added files inside the dir cause an
// explicit refusal rather than silent removal.
const ManifestFilename = ".docops-manifest"

// readManifest returns the sorted list of basenames recorded in the
// dir's manifest. exists is false (and names is nil) when the
// manifest file is absent — first-run upgrades take this path. A
// blank or comment-only manifest yields an empty slice and exists=true.
func readManifest(absDir string) (names []string, exists bool, err error) {
	body, err := os.ReadFile(filepath.Join(absDir, ManifestFilename))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		names = append(names, line)
	}
	sort.Strings(names)
	return names, true, sc.Err()
}

// writeManifest replaces the manifest file with a sorted list of the
// given basenames, prefixed with a comment explaining what the file
// is. Best-effort: callers should warn but not fail if this errors.
func writeManifest(absDir string, names []string) error {
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return err
	}
	sort.Strings(names)
	var buf bytes.Buffer
	buf.WriteString("# docops manifest — files in this directory owned by `docops upgrade`.\n")
	buf.WriteString("# Do not hand-edit; rerun `docops upgrade` to regenerate.\n")
	for _, n := range names {
		buf.WriteString(n)
		buf.WriteByte('\n')
	}
	return os.WriteFile(filepath.Join(absDir, ManifestFilename), buf.Bytes(), 0o644)
}
