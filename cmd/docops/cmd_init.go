package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/logicwind/docops/internal/initter"
	"github.com/logicwind/docops/internal/nextsteps"
	"github.com/logicwind/docops/internal/scaffold"
	"golang.org/x/term"
)

// cmdInit implements `docops init [dir] [--dry-run] [--force] [--no-skills] [--yes]`.
// Exit codes:
//
//	0  scaffold complete (or dry-run rendered successfully, or user aborted)
//	2  usage or filesystem error
func cmdInit(args []string) int {
	// Extract the optional positional [dir] before flag parsing.
	// Go's flag package stops at the first non-flag arg, so we pull the
	// positional out first and pass only flags to fs.Parse.
	positional, flagArgs := splitInitArgs(args)

	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dryRun := fs.Bool("dry-run", false, "print the planned changes without writing")
	force := fs.Bool("force", false, "overwrite files that have drifted from the shipped templates")
	noSkills := fs.Bool("no-skills", false, "skip scaffolding .claude/commands/docops/ and .cursor/commands/docops/")
	yes := fs.Bool("yes", false, "skip the interactive confirm prompt")
	fs.BoolVar(yes, "y", false, "skip the interactive confirm prompt (short form)")
	asJSON := fs.Bool("json", false, "emit a JSON summary (mode + next_steps) instead of human output")
	quiet := fs.Bool("quiet", false, "suppress the closing next-step block")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops init [dir] [--dry-run] [--force] [--no-skills] [--yes] [--json] [--quiet]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	// Resolve target directory.
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops init: %v\n", err)
		return 2
	}

	var targetDir string
	if positional != "" {
		if filepath.IsAbs(positional) {
			targetDir = positional
		} else {
			targetDir = filepath.Join(cwd, positional)
		}
		// If path exists but is not a directory, reject it.
		if info, err := os.Stat(targetDir); err == nil && !info.IsDir() {
			fmt.Fprintf(os.Stderr, "docops init: %s exists but is not a directory\n", targetDir)
			return 2
		}
		// Create directory (and parents) if absent.
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "docops init: create dir %s: %v\n", positional, err)
			return 2
		}
	} else {
		targetDir = cwd
	}

	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops init: resolve path: %v\n", err)
		return 2
	}

	// Print announcement block.
	fmt.Fprintf(os.Stdout, "docops init will scaffold DocOps in %s:\n", absTarget)
	fmt.Fprintln(os.Stdout, "  - docs/{context,decisions,tasks} folders")
	fmt.Fprintln(os.Stdout, "  - docops.yaml at the repo root")
	fmt.Fprintln(os.Stdout, "  - JSON Schemas for editor validation")
	fmt.Fprintln(os.Stdout, "  - AGENTS.md and CLAUDE.md docops blocks (merges into existing content if present)")
	fmt.Fprintln(os.Stdout, "  - A .git/hooks/pre-commit hook (if .git exists)")
	fmt.Fprintln(os.Stdout, "  - /docops:* agent-skill scaffolds under .claude/ and .cursor/")
	fmt.Fprintln(os.Stdout, "Safe to re-run; existing files are never silently overwritten.")

	// Dry-run pass to get the plan (printed to stdout) and count non-skip actions.
	planResult, err := initter.Run(initter.Options{
		Root:     absTarget,
		DryRun:   true,
		Force:    *force,
		NoSkills: *noSkills,
		Out:      os.Stdout,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops init: %v\n", err)
		return 2
	}

	if *dryRun {
		return 0
	}

	// Count non-skip actions to decide whether to prompt.
	nonSkip := 0
	for _, a := range planResult.Actions {
		if a.Kind != "skip" {
			nonSkip++
		}
	}

	// Only prompt when there is something to do and we are on a TTY
	// without --yes. Non-TTY stdin (CI, pipes) skips the prompt automatically.
	if nonSkip > 0 && !*yes && term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stdout, "Proceed? [y/N] ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			answer := strings.TrimSpace(scanner.Text())
			if answer != "y" && answer != "Y" {
				fmt.Fprintln(os.Stdout, "docops init: aborted by user.")
				return 0
			}
		}
	}

	// Execute for real.
	if _, err := initter.Run(initter.Options{
		Root:     absTarget,
		DryRun:   false,
		Force:    *force,
		NoSkills: *noSkills,
		Out:      os.Stdout,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "docops init: %v\n", err)
		return 2
	}

	// Brownfield/greenfield routing for the closing block.
	det := scaffold.DetectBrownfield(absTarget)
	steps := nextsteps.ForInit(nextsteps.Outcome{Brownfield: det.Brownfield})

	if *asJSON {
		mode := "greenfield"
		if det.Brownfield {
			mode = "brownfield"
		}
		out := struct {
			Mode      string           `json:"mode"`
			Signals   []string         `json:"signals"`
			NextSteps []nextsteps.Step `json:"next_steps"`
		}{mode, det.Signals, steps}
		_ = json.NewEncoder(os.Stdout).Encode(out)
		return 0
	}

	if *quiet {
		return 0
	}

	fmt.Fprintln(os.Stdout)
	if det.Brownfield {
		fmt.Fprintf(os.Stdout, "Existing code detected (%s).\n", strings.Join(det.Signals, ", "))
	}
	nextsteps.Render(os.Stdout, steps)
	return 0
}

// splitInitArgs separates the optional positional [dir] from flag arguments.
// The first arg that does not start with "-" is treated as the positional;
// all flags (and their attached values) are collected into flagArgs.
func splitInitArgs(args []string) (positional string, flagArgs []string) {
	for _, a := range args {
		if positional == "" && !strings.HasPrefix(a, "-") {
			positional = a
		} else {
			flagArgs = append(flagArgs, a)
		}
	}
	return positional, flagArgs
}
