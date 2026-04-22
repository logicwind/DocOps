package main

import (
	"fmt"
	"os"

	"github.com/nachiket/docops/internal/version"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		topLevelUsage(os.Stderr)
		return
	}

	switch args[0] {
	case "--version", "-v", "version":
		fmt.Println(version.String())
	case "--help", "-h", "help":
		topLevelUsage(os.Stdout)
	case "validate":
		os.Exit(cmdValidate(args[1:]))
	default:
		fmt.Fprintf(os.Stderr, "docops: unknown command %q\n\n", args[0])
		topLevelUsage(os.Stderr)
		os.Exit(2)
	}
}

func topLevelUsage(w *os.File) {
	fmt.Fprintln(w, "docops — typed project-state substrate for LLM-first development")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "usage: docops <command> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  validate    schema + graph invariants over docs/  (TP-003)")
	fmt.Fprintln(w, "  version     print the build version")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "coming:")
	fmt.Fprintln(w, "  init, index, state, audit, new, status, get, graph, review")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "see `docops <command> --help` for per-command flags.")
}
