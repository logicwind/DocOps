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
	fmt.Fprintln(w, "  init        scaffold DocOps in this repository        (TP-007)")
	fmt.Fprintln(w, "  validate    schema + graph invariants over docs/    (TP-003)")
	fmt.Fprintln(w, "  index       build docs/.index.json enriched graph   (TP-004)")
	fmt.Fprintln(w, "  state       regenerate docs/STATE.md snapshot       (TP-005)")
	fmt.Fprintln(w, "  audit       structural gap punch list                (TP-006)")
	fmt.Fprintln(w, "  new         scaffold a new CTX/ADR/Task document     (TP-008)")
	fmt.Fprintln(w, "  schema      (re)write docs/.docops/schema/*.schema.json  (TP-009)")
	fmt.Fprintln(w, "  refresh     validate + index + state in one pass         (TP-016)")
	fmt.Fprintln(w, "  version     print the build version")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "coming:")
	fmt.Fprintln(w, "  status, get, graph, review")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "see `docops <command> --help` for per-command flags.")
}
