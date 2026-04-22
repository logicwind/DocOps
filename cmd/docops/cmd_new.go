package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/nachiket/docops/internal/config"
	"github.com/nachiket/docops/internal/newdoc"
)

// cmdNew implements `docops new <ctx|adr|task> "title" [flags]`.
// Exit codes:
//
//	0  document created (or --json emitted)
//	2  usage or configuration error
func cmdNew(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: docops new <ctx|adr|task> \"title\" [flags]")
		fmt.Fprintln(os.Stderr, "       docops new ctx --help")
		return 2
	}

	switch args[0] {
	case "ctx":
		return cmdNewCtx(args[1:])
	case "adr":
		return cmdNewADR(args[1:])
	case "task":
		return cmdNewTask(args[1:])
	case "--help", "-h", "help":
		fmt.Fprintln(os.Stdout, "usage: docops new <ctx|adr|task> \"title\" [flags]")
		fmt.Fprintln(os.Stdout, "")
		fmt.Fprintln(os.Stdout, "subcommands:")
		fmt.Fprintln(os.Stdout, "  ctx   create a context document  (CTX-NNN)")
		fmt.Fprintln(os.Stdout, "  adr   create a decision record   (ADR-NNNN)")
		fmt.Fprintln(os.Stdout, "  task  create a task              (TP-NNN)")
		return 0
	default:
		fmt.Fprintf(os.Stderr, "docops new: unknown type %q — must be ctx, adr, or task\n", args[0])
		return 2
	}
}

func cmdNewCtx(args []string) int {
	// Title may be the first positional arg before any flags; extract it so
	// flag.Parse does not stop at it and miss subsequent flag arguments.
	title, flagArgs := extractTitle(args)

	fs := flag.NewFlagSet("new ctx", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	ctxType := fs.String("type", "memo", "context type (prd, design, research, notes, memo, spec, brief)")
	noOpen := fs.Bool("no-open", false, "skip opening $EDITOR after creation")
	asJSON := fs.Bool("json", false, "emit {id, path} JSON instead of human output")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops new ctx \"title\" [--type <type>] [--no-open] [--json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	// Any remaining positional args after flags are appended to the title.
	if rest := strings.Join(fs.Args(), " "); rest != "" {
		if title != "" {
			title += " " + rest
		} else {
			title = rest
		}
	}

	return runNewDoc(newdoc.Options{
		Type:    newdoc.DocTypeCtx,
		Title:   title,
		CtxType: *ctxType,
		NoOpen:  *noOpen,
		JSON:    *asJSON,
	}, *asJSON)
}

func cmdNewADR(args []string) int {
	title, flagArgs := extractTitle(args)

	fs := flag.NewFlagSet("new adr", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	related := fs.String("related", "", "comma-separated related ADR IDs (e.g. ADR-0010,ADR-0004)")
	noOpen := fs.Bool("no-open", false, "skip opening $EDITOR after creation")
	asJSON := fs.Bool("json", false, "emit {id, path} JSON instead of human output")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops new adr \"title\" [--related ADR-0010,ADR-0004] [--no-open] [--json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if rest := strings.Join(fs.Args(), " "); rest != "" {
		if title != "" {
			title += " " + rest
		} else {
			title = rest
		}
	}

	var relatedIDs []string
	if *related != "" {
		for _, r := range strings.Split(*related, ",") {
			if r = strings.TrimSpace(r); r != "" {
				relatedIDs = append(relatedIDs, r)
			}
		}
	}

	return runNewDoc(newdoc.Options{
		Type:    newdoc.DocTypeADR,
		Title:   title,
		Related: relatedIDs,
		NoOpen:  *noOpen,
		JSON:    *asJSON,
	}, *asJSON)
}

func cmdNewTask(args []string) int {
	title, flagArgs := extractTitle(args)

	fs := flag.NewFlagSet("new task", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	requires := fs.String("requires", "", "comma-separated ADR/CTX IDs this task cites (required, ADR-0004)")
	priority := fs.String("priority", "p2", "task priority: p0, p1, p2")
	assignee := fs.String("assignee", "unassigned", "task assignee")
	noOpen := fs.Bool("no-open", false, "skip opening $EDITOR after creation")
	asJSON := fs.Bool("json", false, "emit {id, path} JSON instead of human output")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops new task \"title\" --requires ADR-0020,CTX-003 [--priority p1] [--assignee claude] [--no-open] [--json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if rest := strings.Join(fs.Args(), " "); rest != "" {
		if title != "" {
			title += " " + rest
		} else {
			title = rest
		}
	}

	var requiresIDs []string
	if *requires != "" {
		for _, r := range strings.Split(*requires, ",") {
			if r = strings.TrimSpace(r); r != "" {
				requiresIDs = append(requiresIDs, r)
			}
		}
	}

	return runNewDoc(newdoc.Options{
		Type:     newdoc.DocTypeTask,
		Title:    title,
		Requires: requiresIDs,
		Priority: *priority,
		Assignee: *assignee,
		NoOpen:   *noOpen,
		JSON:     *asJSON,
	}, *asJSON)
}

// extractTitle pulls the first non-flag argument out of args so flag.Parse
// can see all flags regardless of where in the argument list they appear.
// Returns ("", args) when the first arg is a flag (starts with "-").
func extractTitle(args []string) (title string, flagArgs []string) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", args
	}
	return args[0], args[1:]
}

// runNewDoc resolves cwd, calls newdoc.Run, and handles output + exit codes.
func runNewDoc(opts newdoc.Options, asJSON bool) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops new: %v\n", err)
		return 2
	}

	cfg, root, err := config.FindAndLoad(cwd)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "docops new: no docops.yaml found in this directory or any parent — run `docops init` first")
			return 2
		}
		fmt.Fprintf(os.Stderr, "docops new: %v\n", err)
		return 2
	}
	_ = cfg // loaded by newdoc.Run internally; we just needed root
	opts.Root = root

	res, err := newdoc.Run(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops new: %v\n", err)
		return 2
	}

	if asJSON {
		out := struct {
			ID   string `json:"id"`
			Path string `json:"path"`
		}{res.ID, res.Rel}
		enc := json.NewEncoder(os.Stdout)
		_ = enc.Encode(out)
	} else {
		fmt.Fprintf(os.Stdout, "created %s  %s\n", res.ID, res.Rel)
	}
	return 0
}
