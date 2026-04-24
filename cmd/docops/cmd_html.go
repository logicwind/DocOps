package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/htmlviewer"
)

// cmdHtml implements `docops html [--output DIR] [--base-url URL] [--json]`.
// Exit codes:
//
//	0  output written
//	1  runtime error (index failed, write failed)
//	2  usage / bootstrap error
func cmdHtml(args []string) int {
	fs := flag.NewFlagSet("html", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	output := fs.String("output", "", "output directory (default: docs/.html)")
	fs.StringVar(output, "o", "", "output directory (shorthand)")
	baseURL := fs.String("base-url", "", "base href for hosting under a path prefix")
	asJSON := fs.Bool("json", false, "emit JSON summary to stdout")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops html [--output DIR] [--base-url URL] [--json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	idx, root, code := bootstrapIndex("html")
	if code != 0 {
		return code
	}

	cfg, _, err := config.FindAndLoad(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops html: %v\n", err)
		return 2
	}

	out := *output
	if out == "" {
		out = "docs/.html"
	}

	n, err := htmlviewer.Emit(idx, cfg, root, htmlviewer.EmitOptions{
		OutputDir: out,
		BaseURL:   *baseURL,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops html: %v\n", err)
		return 1
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"files_written": n,
			"output_dir":    out,
		})
		return 0
	}

	fmt.Fprintf(os.Stderr, "docops html: wrote %d file(s) to %s\n", n, out)
	fmt.Fprintf(os.Stderr, "  open %s/index.html in a browser — or run `docops serve` for live viewing\n", out)
	return 0
}
