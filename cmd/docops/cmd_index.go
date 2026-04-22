package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/index"
	"github.com/logicwind/docops/internal/loader"
	"github.com/logicwind/docops/internal/validator"
)

// cmdIndex implements `docops index [--json]`.
// Exit codes:
//
//	0  index written (or emitted) successfully
//	1  validation errors prevent indexing
//	2  usage or bootstrap error
func cmdIndex(args []string) int {
	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "write index to stdout instead of the index file")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops index [--json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops index: %v\n", err)
		return 2
	}
	cfg, root, err := config.FindAndLoad(cwd)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "docops index: no docops.yaml found in this directory or any parent — run `docops init` first")
			return 2
		}
		fmt.Fprintf(os.Stderr, "docops index: %v\n", err)
		return 2
	}

	set, err := loader.Load(root, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops index: %v\n", err)
		return 2
	}

	// Run validation first. Refuse to index an invalid repo so consumers
	// never receive an index built from broken source.
	report := validator.Validate(set, cfg)
	if !report.OK() {
		fmt.Fprintf(os.Stderr, "docops index: refusing to index: %d validation error(s); run 'docops validate' to see them\n", len(report.Errors))
		return 1
	}

	idx, err := index.Build(set, cfg, root, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops index: build: %v\n", err)
		return 2
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	if *asJSON {
		if err := enc.Encode(idx); err != nil {
			fmt.Fprintf(os.Stderr, "docops index: encode: %v\n", err)
			return 2
		}
		return 0
	}

	// Write to the configured index path.
	outPath := filepath.Join(root, cfg.Paths.Index)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "docops index: mkdir: %v\n", err)
		return 2
	}

	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops index: create %s: %v\n", outPath, err)
		return 2
	}
	defer f.Close()

	enc = json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(idx); err != nil {
		fmt.Fprintf(os.Stderr, "docops index: write: %v\n", err)
		return 2
	}

	fmt.Fprintf(os.Stderr, "docops index: wrote %s (%d docs)\n", cfg.Paths.Index, len(idx.Docs))
	return 0
}
