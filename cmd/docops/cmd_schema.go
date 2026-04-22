package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/nachiket/docops/internal/config"
	"github.com/nachiket/docops/internal/schema"
)

// cmdSchema implements `docops schema [--stdout]`.
// Exit codes:
//
//	0  schemas written (or printed to stdout)
//	2  bootstrap error (no docops.yaml) or usage error
func cmdSchema(args []string) int {
	fs := flag.NewFlagSet("schema", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	toStdout := fs.Bool("stdout", false, "emit all schemas to stdout as a JSON object {filename: schema} instead of writing files")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops schema [--stdout]")
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
		fmt.Fprintf(os.Stderr, "docops schema: %v\n", err)
		return 2
	}

	cfg, root, err := config.FindAndLoad(cwd)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "docops schema: no docops.yaml found in this directory or any parent — run `docops init` first")
			return 2
		}
		fmt.Fprintf(os.Stderr, "docops schema: %v\n", err)
		return 2
	}

	schemas, err := schema.JSONSchemas(schema.Config{ContextTypes: cfg.ContextTypes})
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops schema: %v\n", err)
		return 2
	}

	if *toStdout {
		// Emit as a JSON object keyed by filename so callers can inspect
		// or pipe to jq. Values are the raw schema objects (not strings)
		// so the result is a well-formed nested JSON document.
		out := make(map[string]json.RawMessage, len(schemas))
		for name, raw := range schemas {
			out[name] = json.RawMessage(raw)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(os.Stderr, "docops schema: encode: %v\n", err)
			return 2
		}
		return 0
	}

	schemaDir := filepath.Join(root, cfg.Paths.Schema)
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "docops schema: mkdir %s: %v\n", cfg.Paths.Schema, err)
		return 2
	}

	// Write in deterministic order so repeated runs are quiet in git diff.
	names := make([]string, 0, len(schemas))
	for name := range schemas {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		dest := filepath.Join(schemaDir, name)
		if err := os.WriteFile(dest, schemas[name], 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "docops schema: write %s: %v\n", name, err)
			return 2
		}
		fmt.Fprintf(os.Stdout, "wrote %s\n", filepath.Join(cfg.Paths.Schema, name))
	}
	return 0
}
