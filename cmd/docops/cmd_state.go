package main

import (
	"errors"
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

// cmdState implements `docops state [--stdout] [--json]`.
// Exit codes:
//
//	0  success
//	1  validation errors prevent state generation
//	2  bootstrap error (no config, unreadable files, etc.)
func cmdState(args []string) int {
	// Manual flag parsing to avoid flag package's stderr noise on --help.
	toStdout := false
	asJSON := false
	for _, a := range args {
		switch a {
		case "--stdout":
			toStdout = true
		case "--json":
			asJSON = true
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "usage: docops state [--stdout] [--json]")
			fmt.Fprintln(os.Stderr, "  --stdout  print STATE.md to stdout instead of writing the file")
			fmt.Fprintln(os.Stderr, "  --json    emit structured JSON to stdout; no file write")
			return 0
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops state: %v\n", err)
		return 2
	}
	cfg, root, err := config.FindAndLoad(cwd)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "docops state: no docops.yaml found in this directory or any parent — run `docops init` first")
			return 2
		}
		fmt.Fprintf(os.Stderr, "docops state: %v\n", err)
		return 2
	}

	set, err := loader.Load(root, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops state: %v\n", err)
		return 2
	}

	// Refuse to generate state from an invalid repo — same guard as cmdIndex.
	report := validator.Validate(set, cfg)
	if !report.OK() {
		fmt.Fprintf(os.Stderr, "docops state: refusing to generate: %d validation error(s); run 'docops validate' to see them\n", len(report.Errors))
		return 1
	}

	now := time.Now()
	idx, err := index.Build(set, cfg, root, now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops state: build index: %v\n", err)
		return 2
	}

	// Collect recent git activity over all doc directories.
	windowDays := cfg.RecentActivityWindowDays
	if windowDays <= 0 {
		windowDays = 7
	}
	since := now.AddDate(0, 0, -windowDays).Format("2006-01-02")
	docPaths := []string{cfg.Paths.Context, cfg.Paths.Decisions, cfg.Paths.Tasks}
	gitActivity, _ := state.RecentGitActivity(root, since, docPaths, 40)
	// Ignore git errors — Compute falls back to index last_touched.

	snap := state.Compute(idx, cfg, gitActivity, now)

	if asJSON {
		out, err := snap.JSON()
		if err != nil {
			fmt.Fprintf(os.Stderr, "docops state: json: %v\n", err)
			return 2
		}
		fmt.Println(string(out))
		return 0
	}

	content := snap.Markdown()

	if toStdout {
		fmt.Print(content)
		return 0
	}

	// Write to the configured state path.
	outPath := filepath.Join(root, cfg.Paths.State)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "docops state: mkdir: %v\n", err)
		return 2
	}
	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "docops state: write %s: %v\n", outPath, err)
		return 2
	}
	fmt.Fprintf(os.Stderr, "docops state: wrote %s\n", cfg.Paths.State)
	return 0
}
