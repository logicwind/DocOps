package htmlviewer

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/index"
)

// Bundle is the single JSON payload the SPA fetches. It is a superset of
// the on-disk docs/.index.json: every IndexedDoc gets its rendered body
// inlined, and STATE.md is inlined at the top level. The SPA therefore
// makes one network request and has everything it needs.
type Bundle struct {
	GeneratedAt string        `json:"generated_at"`
	Version     int           `json:"version"`
	StateMD     string        `json:"state_md,omitempty"`
	Docs        []BundleDoc   `json:"docs"`
}

// BundleDoc embeds IndexedDoc and adds the raw markdown body (with
// frontmatter stripped).
type BundleDoc struct {
	index.IndexedDoc
	Body string `json:"body,omitempty"`
}

// BuildBundle reads each doc's body off disk and returns a viewer bundle.
// State.md is included verbatim if present; missing files are tolerated.
func BuildBundle(idx *index.Index, cfg config.Config, root string) (*Bundle, error) {
	stateSrc := cfg.Paths.State
	if stateSrc == "" {
		stateSrc = "docs/STATE.md"
	}
	if !filepath.IsAbs(stateSrc) {
		stateSrc = filepath.Join(root, stateSrc)
	}
	stateMD := ""
	if b, err := os.ReadFile(stateSrc); err == nil {
		stateMD = string(b)
	}

	docs := make([]BundleDoc, 0, len(idx.Docs))
	for _, d := range idx.Docs {
		body := ""
		src := d.Path
		if !filepath.IsAbs(src) {
			src = filepath.Join(root, src)
		}
		if b, err := os.ReadFile(src); err == nil {
			body = stripFrontmatter(string(b))
		}
		docs = append(docs, BundleDoc{IndexedDoc: d, Body: body})
	}

	return &Bundle{
		GeneratedAt: idx.GeneratedAt,
		Version:     idx.Version,
		StateMD:     stateMD,
		Docs:        docs,
	}, nil
}

// stripFrontmatter removes the leading `---\n...\n---\n` block, if any.
// Preserves the body exactly as authored below the block.
func stripFrontmatter(src string) string {
	if !strings.HasPrefix(src, "---") {
		return src
	}
	rest := src[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return src
	}
	tail := rest[end+4:]
	// Drop a single trailing newline after the closing --- if present.
	if strings.HasPrefix(tail, "\r\n") {
		tail = tail[2:]
	} else if strings.HasPrefix(tail, "\n") {
		tail = tail[1:]
	}
	return tail
}
