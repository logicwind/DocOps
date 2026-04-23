package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/logicwind/docops/internal/scaffold"
	"github.com/logicwind/docops/internal/updatecheck"
	"github.com/logicwind/docops/internal/upgrader"
	"github.com/logicwind/docops/internal/version"
	"golang.org/x/term"
)

// cmdUpgrade implements `docops upgrade [--dry-run] [--yes] [--config]
// [--hook] [--json]`. Exit codes:
//
//	0  upgrade applied (or dry-run rendered, or user aborted)
//	2  bootstrap error (no docops.yaml, safety belt fired, IO error)
func cmdUpgrade(args []string) int {
	fs := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dryRun := fs.Bool("dry-run", false, "print the planned changes without writing")
	cfg := fs.Bool("config", false, "also overwrite docops.yaml from the shipped template")
	hook := fs.Bool("hook", false, "also reinstall the pre-commit hook")
	asJSON := fs.Bool("json", false, "emit a JSON action plan instead of human output")
	yes := fs.Bool("yes", false, "skip the interactive confirm prompt")
	fs.BoolVar(yes, "y", false, "skip the interactive confirm prompt (short form)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops upgrade [--dry-run] [--yes] [--config] [--hook] [--json]")
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
		fmt.Fprintf(os.Stderr, "docops upgrade: %v\n", err)
		return 2
	}

	// Plan first. We always run the planner in dry-run mode internally
	// so we can decide whether to prompt and what to print before
	// committing to disk.
	plan, err := upgrader.Run(upgrader.Options{
		Root:   cwd,
		DryRun: true,
		Config: *cfg,
		Hook:   *hook,
		Out:    io.Discard,
	})
	if err != nil {
		return reportUpgradeError(err)
	}

	if *asJSON {
		emitUpgradeJSON(os.Stdout, plan.Actions)
		if *dryRun {
			return 0
		}
	} else {
		// Pre-flight stale-binary warning. Free on cache hit; one
		// 5s network round-trip on a cold cache. Surfaces only when
		// the user's binary is behind the latest release.
		warnIfStaleBinary(os.Stdout, *yes)
		printUpgradeHeader(os.Stdout, cwd, *cfg, *hook)
		// Re-print the plan via the upgrader's own renderer (we
		// discarded it earlier so we could decide on the prompt
		// first).
		printUpgradePlan(os.Stdout, plan.Actions, true)
	}

	if *dryRun {
		return 0
	}

	nonSkip := 0
	for _, a := range plan.Actions {
		if a.Kind != scaffold.KindSkip {
			nonSkip++
		}
	}

	if !*asJSON && nonSkip > 0 && !*yes && term.IsTerminal(int(os.Stdin.Fd())) {
		if !confirm(os.Stdout, os.Stdin, "Proceed?") {
			fmt.Fprintln(os.Stdout, "docops upgrade: aborted by user.")
			return 0
		}
	}

	// Execute for real. The upgrader prints its own plan again; for
	// JSON callers we already emitted, so route human output to
	// stdout and structured output to a discard sink.
	out := io.Writer(os.Stdout)
	if *asJSON {
		out = io.Discard
	}
	if _, err := upgrader.Run(upgrader.Options{
		Root:   cwd,
		DryRun: false,
		Config: *cfg,
		Hook:   *hook,
		Out:    out,
	}); err != nil {
		return reportUpgradeError(err)
	}
	return 0
}

func reportUpgradeError(err error) int {
	switch {
	case errors.Is(err, upgrader.ErrNoConfig):
		fmt.Fprintln(os.Stderr, "docops upgrade: no docops.yaml in the current directory.")
		fmt.Fprintln(os.Stderr, "                Run `docops init` first to scaffold the project.")
		return 2
	default:
		var unk *upgrader.ErrUnknownFiles
		if errors.As(err, &unk) {
			fmt.Fprintf(os.Stderr, "docops upgrade: %s contains user-added files docops did not write:\n", unk.Dir)
			for _, f := range unk.Files {
				fmt.Fprintf(os.Stderr, "  - %s\n", f)
			}
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Move them one level up (e.g. into .claude/skills/) so docops upgrade can")
			fmt.Fprintln(os.Stderr, "manage its own subdirectory cleanly. See ADR-0021 for the rationale.")
			return 2
		}
		fmt.Fprintf(os.Stderr, "docops upgrade: %v\n", err)
		return 2
	}
}

func printUpgradeHeader(w io.Writer, cwd string, optConfig, optHook bool) {
	fmt.Fprintf(w, "docops upgrade will refresh DocOps-owned scaffolding in %s:\n", cwd)
	fmt.Fprintln(w, "  - .claude/skills/docops/* and .cursor/commands/docops/* (replaced/removed to match the shipped bundle)")
	fmt.Fprintln(w, "  - docs/.docops/schema/*.schema.json (regenerated from docops.yaml)")
	fmt.Fprintln(w, "  - The <!-- docops:* --> block in AGENTS.md and CLAUDE.md (refreshed in place; either file is created if absent)")
	if optConfig {
		fmt.Fprintln(w, "  - docops.yaml (overwritten — --config)")
	}
	if optHook {
		fmt.Fprintln(w, "  - .git/hooks/pre-commit (reinstalled — --hook)")
	}
	fmt.Fprintln(w, "Your docops.yaml, pre-commit hook, and docs/{context,decisions,tasks}/ stay untouched by default.")
}

// printUpgradePlan delegates to the upgrader's Run-time renderer by
// re-running the formatting locally. We duplicate the few lines of
// rendering rather than expose internals because the cmd-level header
// + plan + footer is not the upgrader's responsibility.
func printUpgradePlan(w io.Writer, actions []scaffold.Action, dry bool) {
	var changed, skipped int
	for _, a := range actions {
		if a.Kind == scaffold.KindSkip {
			skipped++
		} else {
			changed++
		}
	}
	verb := "applied"
	if dry {
		verb = "would apply"
	}
	fmt.Fprintf(w, "\ndocops upgrade: %s %d change(s), skipped %d\n", verb, changed, skipped)
	for _, a := range actions {
		sigil, label := upgradeSigilLabel(a)
		fmt.Fprintf(w, "  %s %-40s %s\n", sigil, a.Rel, label)
	}
}

// upgradeSigilLabel mirrors upgrader.upgradeSigil. Kept here to avoid
// exporting an internal helper just for cmd-level rendering; if the
// duplication grows past a third callsite, promote it.
func upgradeSigilLabel(a scaffold.Action) (string, string) {
	switch a.Kind {
	case scaffold.KindMkdir:
		return "+", "(new dir)"
	case scaffold.KindWriteFile:
		if a.Reason == "create" || a.Reason == "install" {
			return "+", "(new)"
		}
		return "~", "(refreshed)"
	case scaffold.KindMergeAgents:
		return "~", "(block refreshed)"
	case scaffold.KindRemove:
		return "-", "(removed)"
	case scaffold.KindSkip:
		return "=", "(up to date)"
	}
	return "?", "(unknown)"
}

func emitUpgradeJSON(w io.Writer, actions []scaffold.Action) {
	type action struct {
		Path string `json:"path"`
		Kind string `json:"kind"`
	}
	out := struct {
		OK      bool     `json:"ok"`
		Actions []action `json:"actions"`
	}{OK: true}
	for _, a := range actions {
		out.Actions = append(out.Actions, action{Path: a.Rel, Kind: jsonKind(a)})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

// jsonKind maps internal action kinds to the user-facing labels
// promised by ADR-0021's --json contract.
func jsonKind(a scaffold.Action) string {
	switch a.Kind {
	case scaffold.KindWriteFile:
		if a.Reason == "create" || a.Reason == "install" {
			return "new"
		}
		return "refreshed"
	case scaffold.KindMergeAgents:
		return "block-refreshed"
	case scaffold.KindRemove:
		return "removed"
	case scaffold.KindSkip:
		return "up-to-date"
	case scaffold.KindMkdir:
		return "new"
	}
	return a.Kind
}

func confirm(w io.Writer, r io.Reader, prompt string) bool {
	fmt.Fprintf(w, "\n%s [y/N] ", prompt)
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return false
	}
	answer := strings.TrimSpace(scanner.Text())
	return answer == "y" || answer == "Y"
}

// warnIfStaleBinary runs the cached update-check and, if the binary
// is behind upstream, prints a warning + interactive prompt. Honors
// `--yes` (proceed without prompting) and non-TTY stdin (proceed
// silently — CI must keep flowing).
func warnIfStaleBinary(w io.Writer, yes bool) {
	res, err := updatecheck.Run(updatecheck.Opts{Local: version.Version})
	if err != nil {
		return
	}
	if res.Status != updatecheck.StatusUpgradeAvailable {
		return
	}
	fmt.Fprintf(w, "Warning: docops %s is installed; %s is available.\n", res.Local, res.Remote)
	fmt.Fprintln(w, "         Run `brew upgrade docops` (or your package manager equivalent)")
	fmt.Fprintln(w, "         before `docops upgrade`, or you'll sync the older templates.")
	fmt.Fprintln(w, "")
	if yes || !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}
	if !confirm(w, os.Stdin, fmt.Sprintf("Continue with %s templates anyway?", res.Local)) {
		fmt.Fprintln(w, "docops upgrade: aborted by user (binary upgrade pending).")
		os.Exit(0)
	}
}
