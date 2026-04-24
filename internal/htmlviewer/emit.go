package htmlviewer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/index"
)

// EmitOptions configures a static emission run.
type EmitOptions struct {
	OutputDir string // e.g. "docs/.html"
	BaseURL   string // optional href for <base>, for hosting under a path prefix
}

// Emit writes the viewer into OutputDir.
//
// Layout — just two files:
//
//	<out>/index.html   — the SPA (with <base href> injected if BaseURL set)
//	<out>/index.json   — viewer bundle: index graph + every doc body + STATE.md
//
// Returns the number of files written. The output directory is created if
// absent; the two files above are overwritten. Nothing else under <out> is
// touched (users may drop in a .nojekyll or README for Pages hosting).
func Emit(idx *index.Index, cfg config.Config, root string, opts EmitOptions) (int, error) {
	if opts.OutputDir == "" {
		return 0, fmt.Errorf("htmlviewer: OutputDir required")
	}
	outAbs := opts.OutputDir
	if !filepath.IsAbs(outAbs) {
		outAbs = filepath.Join(root, outAbs)
	}
	if err := os.MkdirAll(outAbs, 0o755); err != nil {
		return 0, fmt.Errorf("mkdir %s: %w", outAbs, err)
	}

	written := 0

	// SPA.
	spa := SPA
	if opts.BaseURL != "" {
		spa = injectBaseHref(spa, opts.BaseURL)
	}
	if err := os.WriteFile(filepath.Join(outAbs, "index.html"), spa, 0o644); err != nil {
		return written, fmt.Errorf("write index.html: %w", err)
	}
	written++

	// Viewer bundle.
	bundle, err := BuildBundle(idx, cfg, root)
	if err != nil {
		return written, fmt.Errorf("build bundle: %w", err)
	}
	f, err := os.Create(filepath.Join(outAbs, "index.json"))
	if err != nil {
		return written, fmt.Errorf("create index.json: %w", err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(bundle); err != nil {
		f.Close()
		return written, fmt.Errorf("encode index.json: %w", err)
	}
	f.Close()
	written++

	return written, nil
}

// injectBaseHref rewrites the <head> of the SPA to include <base href="...">.
// If a <base> tag already exists we leave the bytes untouched.
var headOpen = regexp.MustCompile(`(?i)<head[^>]*>`)

func injectBaseHref(spa []byte, href string) []byte {
	if regexp.MustCompile(`(?i)<base\s`).Match(spa) {
		return spa
	}
	tag := []byte(fmt.Sprintf(`<base href=%q>`, href))
	return headOpen.ReplaceAllFunc(spa, func(match []byte) []byte {
		out := make([]byte, 0, len(match)+len(tag))
		out = append(out, match...)
		out = append(out, tag...)
		return out
	})
}
