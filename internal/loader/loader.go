// Package loader walks a DocOps project's `docs/` tree and produces a
// unified DocSet — every CTX, ADR, and TP file parsed into typed
// frontmatter plus the path/ID metadata that graph checks rely on.
package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nachiket/docops/internal/config"
	"github.com/nachiket/docops/internal/schema"
)

// Doc is the uniform wrapper around a single markdown document. Only one
// of the three typed pointers is set based on Kind.
type Doc struct {
	Kind schema.Kind
	ID   string
	Path string // relative to project root

	Context *schema.Context
	ADR     *schema.ADR
	Task    *schema.Task

	// ParseErr is non-nil when frontmatter could not be decoded. The Doc
	// is still retained (with ID from filename) so callers can report
	// the problem without losing the file from the set.
	ParseErr error
}

// Supersedes returns the IDs this doc supersedes regardless of kind.
func (d Doc) Supersedes() []string {
	switch d.Kind {
	case schema.KindContext:
		if d.Context != nil {
			return d.Context.Supersedes
		}
	case schema.KindADR:
		if d.ADR != nil {
			return d.ADR.Supersedes
		}
	}
	return nil
}

// Related returns related-edge targets (currently ADR-only).
func (d Doc) Related() []string {
	if d.Kind == schema.KindADR && d.ADR != nil {
		return d.ADR.Related
	}
	return nil
}

// Requires returns `requires` IDs (Task-only).
func (d Doc) Requires() []string {
	if d.Kind == schema.KindTask && d.Task != nil {
		return d.Task.Requires
	}
	return nil
}

// DependsOn returns `depends_on` IDs (Task-only).
func (d Doc) DependsOn() []string {
	if d.Kind == schema.KindTask && d.Task != nil {
		return d.Task.DependsOn
	}
	return nil
}

// Status returns the status string when the kind has one, else "".
func (d Doc) Status() string {
	switch d.Kind {
	case schema.KindADR:
		if d.ADR != nil {
			return d.ADR.Status
		}
	case schema.KindTask:
		if d.Task != nil {
			return d.Task.Status
		}
	}
	return ""
}

// DocSet is the collection of every DocOps doc in a project, indexed by ID.
type DocSet struct {
	Root string          // absolute project root (where docops.yaml lives)
	Docs map[string]*Doc // keyed by ID (e.g. "ADR-0012")
	// Order preserves discovery order so reports are deterministic.
	Order []string
}

// Get returns the doc with the given ID, or nil if absent.
func (s *DocSet) Get(id string) *Doc { return s.Docs[id] }

// Has reports whether the set contains a doc with the given ID.
func (s *DocSet) Has(id string) bool { _, ok := s.Docs[id]; return ok }

// Load walks the three doc directories named in cfg and returns a DocSet
// rooted at root. Directories that do not exist yet are tolerated (a
// freshly-init'd repo has no decisions/ folder until the first ADR).
func Load(root string, cfg config.Config) (*DocSet, error) {
	set := &DocSet{Root: root, Docs: map[string]*Doc{}}

	dirs := []struct {
		path string
		kind schema.Kind
	}{
		{cfg.Paths.Context, schema.KindContext},
		{cfg.Paths.Decisions, schema.KindADR},
		{cfg.Paths.Tasks, schema.KindTask},
	}

	for _, d := range dirs {
		abs := filepath.Join(root, d.path)
		entries, err := os.ReadDir(abs)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", abs, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			doc, err := loadFile(filepath.Join(abs, e.Name()), filepath.Join(d.path, e.Name()), d.kind)
			if err != nil {
				return nil, err
			}
			if doc == nil {
				continue // filename did not match any kind — skip (e.g. STATE.md in decisions/)
			}
			if existing, dup := set.Docs[doc.ID]; dup {
				return nil, fmt.Errorf("duplicate ID %s: %s and %s", doc.ID, existing.Path, doc.Path)
			}
			set.Docs[doc.ID] = doc
			set.Order = append(set.Order, doc.ID)
		}
	}
	return set, nil
}

// loadFile reads a single markdown file, parses its frontmatter into the
// expected kind, and returns a *Doc. A nil doc (with nil error) means the
// filename did not identify a known kind and the file should be ignored.
func loadFile(absPath, relPath string, expected schema.Kind) (*Doc, error) {
	name := filepath.Base(absPath)
	kind, ok := schema.KindFromFilename(name)
	if !ok {
		return nil, nil
	}
	if kind != expected {
		// File lives in the wrong directory for its kind — treat as an
		// error so authors fix the layout rather than have it silently
		// ignored by graph checks.
		return nil, fmt.Errorf("%s: %s doc in %s/ directory (expected %s)", relPath, kind.Prefix(), filepath.Dir(relPath), expected.Prefix())
	}
	id := idFromFilename(name, kind)

	raw, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", relPath, err)
	}
	fm, _, err := schema.SplitFrontmatter(raw)
	if err != nil {
		return &Doc{Kind: kind, ID: id, Path: relPath, ParseErr: err}, nil
	}

	doc := &Doc{Kind: kind, ID: id, Path: relPath}
	switch kind {
	case schema.KindContext:
		c, err := schema.ParseContext(fm)
		if err != nil {
			doc.ParseErr = err
		} else {
			doc.Context = &c
		}
	case schema.KindADR:
		a, err := schema.ParseADR(fm)
		if err != nil {
			doc.ParseErr = err
		} else {
			doc.ADR = &a
		}
	case schema.KindTask:
		t, err := schema.ParseTask(fm)
		if err != nil {
			doc.ParseErr = err
		} else {
			doc.Task = &t
		}
	}
	return doc, nil
}

// idFromFilename extracts "ADR-0012" from "ADR-0012-foo.md".
func idFromFilename(name string, kind schema.Kind) string {
	stem := strings.TrimSuffix(name, ".md")
	// Expected shape: PREFIX-NUMBER[-rest]. Split on the second dash.
	parts := strings.SplitN(stem, "-", 3)
	if len(parts) < 2 {
		return stem
	}
	return parts[0] + "-" + parts[1]
}
