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
	"github.com/logicwind/docops/internal/state"
	"github.com/logicwind/docops/internal/validator"
)

// refreshStep is one step in the refresh pipeline.
type refreshStep struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Errors   int    `json:"errors,omitempty"`
	Warnings int    `json:"warnings,omitempty"`
	Files    int    `json:"files,omitempty"`
	Path     string `json:"path,omitempty"`
	Skipped  bool   `json:"skipped,omitempty"`
}

// cmdRefresh implements `docops refresh [--json]`.
// Runs validate → index → state in sequence, stopping at the first error.
// Exit codes:
//
//	0  all steps OK
//	1  validate failed (index + state were skipped)
//	2  bootstrap error (no docops.yaml, unreadable files, etc.)
func cmdRefresh(args []string) int {
	fs := flag.NewFlagSet("refresh", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "emit aggregate JSON status instead of human output")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops refresh [--json]")
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
		fmt.Fprintf(os.Stderr, "docops refresh: %v\n", err)
		return 2
	}
	cfg, root, err := config.FindAndLoad(cwd)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "docops refresh: no docops.yaml found in this directory or any parent — run `docops init` first")
			return 2
		}
		fmt.Fprintf(os.Stderr, "docops refresh: %v\n", err)
		return 2
	}

	set, err := loader.Load(root, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops refresh: %v\n", err)
		return 2
	}

	// --- step 1: validate ---
	report := validator.Validate(set, cfg)
	validateOK := report.OK()

	vStep := refreshStep{
		Name:     "validate",
		OK:       validateOK,
		Errors:   len(report.Errors),
		Warnings: len(report.Warnings),
		Files:    len(report.Files),
	}

	if !*asJSON {
		if validateOK {
			fmt.Fprintf(os.Stdout, "validate: OK (%d docs, %d errors, %d warnings)\n",
				len(report.Files), len(report.Errors), len(report.Warnings))
		} else {
			for _, f := range report.Errors {
				fmt.Fprintln(os.Stderr, formatFinding(f))
			}
			for _, f := range report.Warnings {
				fmt.Fprintln(os.Stderr, formatFinding(f))
			}
			fmt.Fprintf(os.Stdout, "validate: FAIL (%d errors, %d warnings)\n",
				len(report.Errors), len(report.Warnings))
		}
	}

	if !validateOK {
		iStep := refreshStep{Name: "index", OK: false, Skipped: true}
		sStep := refreshStep{Name: "state", OK: false, Skipped: true}
		if *asJSON {
			printRefreshJSON(false, []refreshStep{vStep, iStep, sStep})
		} else {
			fmt.Fprintln(os.Stdout, "index:    SKIP (validate failed)")
			fmt.Fprintln(os.Stdout, "state:    SKIP (validate failed)")
			fmt.Fprintln(os.Stdout, "docops refresh: FAIL")
		}
		return 1
	}

	// --- step 2: index ---
	now := time.Now()
	idx, err := index.Build(set, cfg, root, now)
	if err != nil {
		iStep := refreshStep{Name: "index", OK: false}
		sStep := refreshStep{Name: "state", OK: false, Skipped: true}
		if *asJSON {
			printRefreshJSON(false, []refreshStep{vStep, iStep, sStep})
		} else {
			fmt.Fprintf(os.Stderr, "docops refresh: index build: %v\n", err)
			fmt.Fprintln(os.Stdout, "index:    FAIL")
			fmt.Fprintln(os.Stdout, "state:    SKIP (index failed)")
			fmt.Fprintln(os.Stdout, "docops refresh: FAIL")
		}
		return 1
	}

	indexPath := filepath.Join(root, cfg.Paths.Index)
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "docops refresh: index mkdir: %v\n", err)
		return 2
	}
	fh, err := os.Create(indexPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops refresh: index create: %v\n", err)
		return 2
	}
	enc := json.NewEncoder(fh)
	enc.SetIndent("", "  ")
	if encErr := enc.Encode(idx); encErr != nil {
		_ = fh.Close()
		fmt.Fprintf(os.Stderr, "docops refresh: index write: %v\n", encErr)
		return 2
	}
	_ = fh.Close()

	iStep := refreshStep{Name: "index", OK: true, Path: cfg.Paths.Index, Files: len(idx.Docs)}
	if !*asJSON {
		fmt.Fprintf(os.Stdout, "index:    OK (wrote %s)\n", cfg.Paths.Index)
	}

	// --- step 3: state ---
	windowDays := cfg.RecentActivityWindowDays
	if windowDays <= 0 {
		windowDays = 7
	}
	since := now.AddDate(0, 0, -windowDays).Format("2006-01-02")
	docPaths := []string{cfg.Paths.Context, cfg.Paths.Decisions, cfg.Paths.Tasks}
	gitActivity, _ := state.RecentGitActivity(root, since, docPaths, 40)

	snap := state.Compute(idx, cfg, gitActivity, now)
	content := snap.Markdown()

	statePath := filepath.Join(root, cfg.Paths.State)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "docops refresh: state mkdir: %v\n", err)
		return 2
	}
	if err := os.WriteFile(statePath, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "docops refresh: state write: %v\n", err)
		return 2
	}

	sStep := refreshStep{Name: "state", OK: true, Path: cfg.Paths.State}
	if !*asJSON {
		fmt.Fprintf(os.Stdout, "state:    OK (wrote %s)\n", cfg.Paths.State)
		fmt.Fprintln(os.Stdout, "docops refresh: OK")
	} else {
		printRefreshJSON(true, []refreshStep{vStep, iStep, sStep})
	}
	return 0
}

// printRefreshJSON emits the aggregate JSON report to stdout.
func printRefreshJSON(ok bool, steps []refreshStep) {
	out := struct {
		OK    bool          `json:"ok"`
		Steps []refreshStep `json:"steps"`
	}{OK: ok, Steps: steps}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
