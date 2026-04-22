package main

import (
	"fmt"
	"os"

	"github.com/logicwind/docops/internal/version"
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
	case "init":
		os.Exit(cmdInit(args[1:]))
	case "validate":
		os.Exit(cmdValidate(args[1:]))
	case "index":
		os.Exit(cmdIndex(args[1:]))
	case "state":
		os.Exit(cmdState(args[1:]))
	case "audit":
		os.Exit(cmdAudit(args[1:]))
	case "new":
		os.Exit(cmdNew(args[1:]))
	case "schema":
		os.Exit(cmdSchema(args[1:]))
	case "refresh":
		os.Exit(cmdRefresh(args[1:]))
	case "get":
		os.Exit(cmdGet(args[1:]))
	case "list":
		os.Exit(cmdList(args[1:]))
	case "graph":
		os.Exit(cmdGraph(args[1:]))
	case "next":
		os.Exit(cmdNext(args[1:]))
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
	fmt.Fprintln(w, "  init        scaffold DocOps in this repository")
	fmt.Fprintln(w, "  validate    schema + graph invariants over docs/")
	fmt.Fprintln(w, "  index       build docs/.index.json enriched graph")
	fmt.Fprintln(w, "  state       regenerate docs/STATE.md snapshot")
	fmt.Fprintln(w, "  audit       structural gap punch list")
	fmt.Fprintln(w, "  new         scaffold a new CTX/ADR/Task document")
	fmt.Fprintln(w, "  schema      (re)write docs/.docops/schema/*.schema.json")
	fmt.Fprintln(w, "  refresh     validate + index + state in one pass")
	fmt.Fprintln(w, "  get         look up one doc by ID")
	fmt.Fprintln(w, "  list        list docs with optional filters")
	fmt.Fprintln(w, "  graph       typed edge graph from a starting doc")
	fmt.Fprintln(w, "  next        recommend the next task to work on")
	fmt.Fprintln(w, "  version     print the build version")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "see `docops <command> --help` for per-command flags.")
}
