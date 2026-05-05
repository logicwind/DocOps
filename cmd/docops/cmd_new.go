package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/newdoc"
	"github.com/logicwind/docops/internal/nextsteps"
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
	body := fs.String("body", "", "read the document body from stdin (-) or literal string; replaces the default stub")
	bodyFile := fs.String("body-file", "", "read the document body from <path>; replaces the default stub")
	quiet := fs.Bool("quiet", false, "suppress the closing next-step block")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops new ctx \"title\" [--type <type>] [--no-open] [--json] [--body -|text] [--body-file <path>] [--quiet]")
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

	bodyReader, code := resolveBody(*body, *bodyFile)
	if code != 0 {
		return code
	}

	return runNewDoc(newdoc.Options{
		Type:       newdoc.DocTypeCtx,
		Title:      title,
		CtxType:    *ctxType,
		NoOpen:     *noOpen,
		JSON:       *asJSON,
		BodyReader: bodyReader,
	}, *asJSON, *quiet)
}

func cmdNewADR(args []string) int {
	title, flagArgs := extractTitle(args)

	fs := flag.NewFlagSet("new adr", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	related := fs.String("related", "", "comma-separated related ADR IDs (e.g. ADR-0010,ADR-0004)")
	noOpen := fs.Bool("no-open", false, "skip opening $EDITOR after creation")
	asJSON := fs.Bool("json", false, "emit {id, path} JSON instead of human output")
	body := fs.String("body", "", "read the document body from stdin (-) or literal string; replaces the default stub")
	bodyFile := fs.String("body-file", "", "read the document body from <path>; replaces the default stub")
	quiet := fs.Bool("quiet", false, "suppress the closing next-step block")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops new adr \"title\" [--related ADR-0010,ADR-0004] [--no-open] [--json] [--body -|text] [--body-file <path>] [--quiet]")
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

	bodyReader, code := resolveBody(*body, *bodyFile)
	if code != 0 {
		return code
	}

	return runNewDoc(newdoc.Options{
		Type:       newdoc.DocTypeADR,
		Title:      title,
		Related:    relatedIDs,
		NoOpen:     *noOpen,
		JSON:       *asJSON,
		BodyReader: bodyReader,
	}, *asJSON, *quiet)
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
	body := fs.String("body", "", "read the document body from stdin (-) or literal string; replaces the default stub")
	bodyFile := fs.String("body-file", "", "read the document body from <path>; replaces the default stub")
	quiet := fs.Bool("quiet", false, "suppress the closing next-step block")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops new task \"title\" --requires ADR-0020,CTX-003 [--priority p1] [--assignee claude] [--no-open] [--json] [--body -|text] [--body-file <path>] [--quiet]")
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

	bodyReader, code := resolveBody(*body, *bodyFile)
	if code != 0 {
		return code
	}

	return runNewDoc(newdoc.Options{
		Type:       newdoc.DocTypeTask,
		Title:      title,
		Requires:   requiresIDs,
		Priority:   *priority,
		Assignee:   *assignee,
		NoOpen:     *noOpen,
		JSON:       *asJSON,
		BodyReader: bodyReader,
	}, *asJSON, *quiet)
}

// resolveBody resolves the --body / --body-file flag pair into an io.Reader.
// Returns (nil, 0) when neither flag is set (stub body will be used).
// Returns (reader, 0) on success, or (nil, 2) on usage/IO error.
func resolveBody(body, bodyFile string) (io.Reader, int) {
	if body != "" && bodyFile != "" {
		fmt.Fprintln(os.Stderr, "docops new: --body and --body-file are mutually exclusive")
		return nil, 2
	}
	if body == "-" {
		// Read from stdin. On a terminal with no piped content this would
		// block, so we detect that and exit cleanly.
		if isTerminalStdin() {
			fmt.Fprintln(os.Stderr, "docops new: no body on stdin; pipe content or use --body-file")
			return nil, 2
		}
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "docops new: read stdin: %v\n", err)
			return nil, 2
		}
		return bytes.NewReader(data), 0
	}
	if body != "" {
		return strings.NewReader(body), 0
	}
	if bodyFile != "" {
		data, err := os.ReadFile(bodyFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "docops new: --body-file %s: %v\n", bodyFile, err)
			return nil, 2
		}
		return bytes.NewReader(data), 0
	}
	return nil, 0
}

// isTerminalStdin reports whether stdin is an interactive terminal.
// Inlined here to avoid importing golang.org/x/term into newdoc package.
func isTerminalStdin() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	// ModeCharDevice is set for a real terminal; pipes and files are not.
	return (fi.Mode() & os.ModeCharDevice) != 0
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
func runNewDoc(opts newdoc.Options, asJSON, quiet bool) int {
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

	steps := suggestForNew(opts.Type, res.ID)

	if asJSON {
		out := struct {
			ID        string           `json:"id"`
			Path      string           `json:"path"`
			NextSteps []nextsteps.Step `json:"next_steps,omitempty"`
		}{res.ID, res.Rel, steps}
		enc := json.NewEncoder(os.Stdout)
		_ = enc.Encode(out)
		return 0
	}

	fmt.Fprintf(os.Stdout, "created %s  %s\n", res.ID, res.Rel)
	if !quiet {
		fmt.Fprintln(os.Stdout)
		nextsteps.Render(os.Stdout, steps)
	}
	return 0
}

// suggestForNew picks the affordance set keyed off the doc type just
// created. Task ID is the most useful payload to thread through.
func suggestForNew(t newdoc.DocType, id string) []nextsteps.Step {
	switch t {
	case newdoc.DocTypeCtx:
		return nextsteps.ForNewCTX(nextsteps.Outcome{ID: id})
	case newdoc.DocTypeADR:
		return nextsteps.ForNewADR(nextsteps.Outcome{ID: id})
	case newdoc.DocTypeTask:
		return nextsteps.ForNewTask(nextsteps.Outcome{ID: id})
	default:
		return nil
	}
}
