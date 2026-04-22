package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nachiket/docops/internal/config"
	"github.com/nachiket/docops/internal/loader"
	"github.com/nachiket/docops/internal/validator"
)

// cmdValidate implements `docops validate [--json] [--only PATH]`.
// Exit codes:
//   0  valid (no errors; warnings allowed)
//   1  validation failures present
//   2  usage or bootstrap error (no config found, unreadable file, etc.)
func cmdValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "emit the report as JSON")
	only := fs.String("only", "", "validate only the doc at this path (still loads the full project for graph checks)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops validate [--json] [--only <path>]")
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
		fmt.Fprintf(os.Stderr, "docops validate: %v\n", err)
		return 2
	}
	cfg, root, err := config.FindAndLoad(cwd)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "docops validate: no docops.yaml found in this directory or any parent — run `docops init` first")
			return 2
		}
		fmt.Fprintf(os.Stderr, "docops validate: %v\n", err)
		return 2
	}

	set, err := loader.Load(root, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops validate: %v\n", err)
		return 2
	}

	report := validator.Validate(set, cfg)

	if *only != "" {
		keep, err := filterForOnly(*only, root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "docops validate: --only: %v\n", err)
			return 2
		}
		report = filterReport(report, keep)
	}

	if *asJSON {
		emitJSON(report)
	} else {
		emitHuman(report, root)
	}

	if !report.OK() {
		return 1
	}
	return 0
}

func filterForOnly(only, root string) (string, error) {
	abs, err := filepath.Abs(only)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); err != nil {
		return "", err
	}
	return rel, nil
}

// filterReport narrows the report to findings on the chosen path without
// dropping the file list (which stays for context in --json mode).
func filterReport(r validator.Report, path string) validator.Report {
	out := validator.Report{Files: r.Files}
	for _, f := range r.Errors {
		if f.Path == path {
			out.Errors = append(out.Errors, f)
		}
	}
	for _, f := range r.Warnings {
		if f.Path == path {
			out.Warnings = append(out.Warnings, f)
		}
	}
	return out
}

func emitJSON(r validator.Report) {
	// Wrap in an object that is explicit about pass/fail so JSON consumers
	// do not have to re-derive it from len(errors).
	out := struct {
		OK       bool                `json:"ok"`
		Files    []string            `json:"files"`
		Errors   []validator.Finding `json:"errors"`
		Warnings []validator.Finding `json:"warnings"`
	}{
		OK:       r.OK(),
		Files:    r.Files,
		Errors:   r.Errors,
		Warnings: r.Warnings,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func emitHuman(r validator.Report, root string) {
	for _, f := range r.Errors {
		fmt.Fprintln(os.Stderr, formatFinding(f))
	}
	for _, f := range r.Warnings {
		fmt.Fprintln(os.Stderr, formatFinding(f))
	}
	fmt.Fprintf(os.Stderr, "\nvalidated %d doc(s): %d error(s), %d warning(s)\n",
		len(r.Files), len(r.Errors), len(r.Warnings))
	if r.OK() && len(r.Warnings) == 0 {
		fmt.Fprintln(os.Stderr, "all clear ✓")
	}
	_ = root // reserved for future relative-path formatting
}

func formatFinding(f validator.Finding) string {
	prefix := "error"
	if f.Severity == validator.SeverityWarn {
		prefix = "warn "
	}
	loc := f.Path
	if loc == "" {
		loc = f.ID
	}
	field := f.Field
	if field != "" {
		field = " " + field
	}
	return fmt.Sprintf("%s %s [%s]%s: %s", prefix, loc, f.Rule, field, f.Message)
}
