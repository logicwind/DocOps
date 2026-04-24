package htmlviewer

import (
	"encoding/json"
	"fmt"
	"io"
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

// Emit writes the SPA plus the data it needs into OutputDir.
//
// Layout:
//
//	<out>/index.html         — SPA (with <base href> injected if BaseURL set)
//	<out>/index.json         — serialized index
//	<out>/state.md           — contents of cfg.Paths.State if present, else ""
//	<out>/raw/<path>         — verbatim copy of each source markdown file
//
// Returns the number of files written. The output directory is created if
// absent; existing files in it are overwritten but files outside the four
// categories above are left alone (this is deliberately not a hard wipe —
// users may commit extra things like .nojekyll or a README).
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

	// 1. SPA.
	spa := SPA
	if opts.BaseURL != "" {
		spa = injectBaseHref(spa, opts.BaseURL)
	}
	if err := os.WriteFile(filepath.Join(outAbs, "index.html"), spa, 0o644); err != nil {
		return written, fmt.Errorf("write index.html: %w", err)
	}
	written++

	// 2. index.json.
	idxPath := filepath.Join(outAbs, "index.json")
	f, err := os.Create(idxPath)
	if err != nil {
		return written, fmt.Errorf("create index.json: %w", err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(idx); err != nil {
		f.Close()
		return written, fmt.Errorf("encode index.json: %w", err)
	}
	f.Close()
	written++

	// 3. state.md (best-effort — if the repo hasn't run `docops state` yet,
	// skip without error).
	stateSrc := cfg.Paths.State
	if stateSrc == "" {
		stateSrc = "docs/STATE.md"
	}
	if !filepath.IsAbs(stateSrc) {
		stateSrc = filepath.Join(root, stateSrc)
	}
	if b, err := os.ReadFile(stateSrc); err == nil {
		if err := os.WriteFile(filepath.Join(outAbs, "state.md"), b, 0o644); err != nil {
			return written, fmt.Errorf("write state.md: %w", err)
		}
		written++
	}

	// 4. raw/* — copy each source markdown file preserving its relative path.
	rawDir := filepath.Join(outAbs, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		return written, fmt.Errorf("mkdir raw: %w", err)
	}
	for _, doc := range idx.Docs {
		if doc.Path == "" {
			continue
		}
		src := doc.Path
		if !filepath.IsAbs(src) {
			src = filepath.Join(root, src)
		}
		dst := filepath.Join(rawDir, doc.Path)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return written, fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
		}
		if err := copyFile(src, dst); err != nil {
			return written, fmt.Errorf("copy %s: %w", doc.Path, err)
		}
		written++
	}

	return written, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
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
