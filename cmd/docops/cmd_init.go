package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/nachiket/docops/internal/initter"
)

// cmdInit implements `docops init [--dry-run] [--force] [--no-skills]`.
// Exit codes:
//
//	0  scaffold complete (or dry-run rendered successfully)
//	2  usage or filesystem error
func cmdInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dryRun := fs.Bool("dry-run", false, "print the planned changes without writing")
	force := fs.Bool("force", false, "overwrite files that have drifted from the shipped templates")
	noSkills := fs.Bool("no-skills", false, "skip scaffolding .claude/skills/docops/ and .cursor/commands/docops/")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops init [--dry-run] [--force] [--no-skills]")
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
		fmt.Fprintf(os.Stderr, "docops init: %v\n", err)
		return 2
	}

	if _, err := initter.Run(initter.Options{
		Root:     cwd,
		DryRun:   *dryRun,
		Force:    *force,
		NoSkills: *noSkills,
		Out:      os.Stdout,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "docops init: %v\n", err)
		return 2
	}
	return 0
}
