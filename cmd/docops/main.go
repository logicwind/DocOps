package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/nachiket/docops/internal/version"
)

func main() {
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&showVersion, "v", false, "print version and exit (shorthand)")
	flag.Usage = usage
	flag.Parse()

	if showVersion {
		fmt.Println(version.String())
		return
	}

	if flag.NArg() == 0 {
		usage()
		return
	}

	fmt.Fprintf(os.Stderr, "docops: unknown command %q\n", flag.Arg(0))
	fmt.Fprintln(os.Stderr, "subcommands land in subsequent tasks (see docs/tasks/)")
	os.Exit(2)
}

func usage() {
	fmt.Fprintln(os.Stderr, "docops — typed project-state substrate for LLM-first development")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "usage: docops [--version] <command> [args]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "This build is the phase-1 scaffold (TP-001). Commands will be added by:")
	fmt.Fprintln(os.Stderr, "  TP-002  schemas")
	fmt.Fprintln(os.Stderr, "  TP-003  validate")
	fmt.Fprintln(os.Stderr, "  TP-004  index")
	fmt.Fprintln(os.Stderr, "  TP-005  state")
	fmt.Fprintln(os.Stderr, "  TP-006  audit")
	fmt.Fprintln(os.Stderr, "  TP-007  init")
	fmt.Fprintln(os.Stderr, "  TP-008  new")
}
