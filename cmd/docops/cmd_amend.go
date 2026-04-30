package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/logicwind/docops/internal/amender"
	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/schema"
)

// cmdAmend implements `docops amend <ADR-ID> [flags]`. See ADR-0025 for the
// design and TP-026 for the acceptance criteria.
//
// Exit codes:
//
//	0  amendment written
//	2  usage / configuration error
//	1  semantic failure (e.g. marker substring not found / ambiguous)
func cmdAmend(args []string) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printAmendUsage(os.Stdout)
		if len(args) == 0 {
			return 2
		}
		return 0
	}

	adrID, flagArgs := extractTitle(args) // first positional = ADR id

	fs := flag.NewFlagSet("amend", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	kind := fs.String("kind", "", strings.Join(schema.AmendmentKinds, " | ")+"  (required)")
	summary := fs.String("summary", "", "one-line human summary (required)")
	by := fs.String("by", "", "author handle (defaults to $DOCOPS_USER, git user.name, or $USER)")
	ref := fs.String("ref", "", "optional follow-up ADR id, PR URL, issue ref, or task id")
	markerAt := fs.String("marker-at", "", "literal substring in the ADR body to receive the [AMENDED ...] marker; exact match required")
	body := fs.String("body", "", "amendment body — literal text or `-` for stdin")
	bodyFile := fs.String("body-file", "", "amendment body read from <path>")
	noOpen := fs.Bool("no-open", false, "skip opening $EDITOR after writing")
	asJSON := fs.Bool("json", false, "emit {adr, amendment_index, path} JSON instead of human output")
	dateOverride := fs.String("date", "", "override amendment date (YYYY-MM-DD); defaults to today")
	var sectionsFlag stringSliceFlag
	fs.Var(&sectionsFlag, "section", "repeatable; affects_sections entry")

	fs.Usage = func() {
		printAmendUsage(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if adrID == "" {
		fmt.Fprintln(os.Stderr, "docops amend: ADR id is required as the first argument")
		return 2
	}

	bodyReader, code := resolveBodyAmend(*body, *bodyFile)
	if code != 0 {
		return code
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops amend: %v\n", err)
		return 2
	}
	_, root, err := config.FindAndLoad(cwd)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "docops amend: no docops.yaml found — run `docops init` first")
			return 2
		}
		fmt.Fprintf(os.Stderr, "docops amend: %v\n", err)
		return 2
	}

	resolvedBy := *by
	if resolvedBy == "" {
		resolvedBy = gitUserName()
	}

	res, err := amender.Run(amender.Options{
		Root:            root,
		ADRID:           adrID,
		Kind:            *kind,
		Summary:         *summary,
		By:              resolvedBy,
		Ref:             *ref,
		AffectsSections: []string(sectionsFlag),
		MarkerAt:        *markerAt,
		BodyReader:      bodyReader,
		Date:            *dateOverride,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops amend: %v\n", err)
		var notFound *amender.MarkerNotFoundError
		var ambig *amender.MarkerAmbiguousError
		if errors.As(err, &notFound) || errors.As(err, &ambig) {
			return 1
		}
		return 2
	}

	if *asJSON {
		out := struct {
			ADR             string `json:"adr"`
			AmendmentIndex  int    `json:"amendment_index"`
			Path            string `json:"path"`
			MarkerInserted  bool   `json:"marker_inserted"`
			SectionCreated  bool   `json:"section_created"`
		}{res.ADRID, res.AmendmentIndex, res.Rel, res.MarkerInserted, res.SectionCreated}
		_ = json.NewEncoder(os.Stdout).Encode(out)
	} else {
		fmt.Fprintf(os.Stdout, "amended %s  %s  (entry %d)\n", res.ADRID, res.Rel, res.AmendmentIndex)
		if res.MarkerInserted {
			fmt.Fprintln(os.Stdout, "  marker inserted in body")
		}
		if res.SectionCreated {
			fmt.Fprintln(os.Stdout, "  ## Amendments section created")
		}
		fmt.Fprintln(os.Stdout, "  tip: run `docops refresh` to rebuild .index.json + STATE.md")
	}

	if !*noOpen && *asJSON == false {
		openEditor(res.Path)
	}
	return 0
}

func printAmendUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: docops amend <ADR-ID> --kind <kind> --summary \"...\" [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Append an amendment entry per ADR-0025. The ADR's decision body is")
	fmt.Fprintln(w, "never rewritten; amendments are additive frontmatter + an `## Amendments`")
	fmt.Fprintln(w, "subsection. If --marker-at is provided, an inline [AMENDED YYYY-MM-DD kind]")
	fmt.Fprintln(w, "marker is inserted right after the matching substring (must be unique).")
}

// resolveBodyAmend mirrors resolveBody from cmd_new but reports the right
// command name in errors. Returns (nil, 0) when no body flag is set so the
// amender writes its default stub.
func resolveBodyAmend(body, bodyFile string) (io.Reader, int) {
	if body != "" && bodyFile != "" {
		fmt.Fprintln(os.Stderr, "docops amend: --body and --body-file are mutually exclusive")
		return nil, 2
	}
	if body == "-" {
		if isTerminalStdin() {
			fmt.Fprintln(os.Stderr, "docops amend: no body on stdin; pipe content or use --body-file")
			return nil, 2
		}
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "docops amend: read stdin: %v\n", err)
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
			fmt.Fprintf(os.Stderr, "docops amend: --body-file %s: %v\n", bodyFile, err)
			return nil, 2
		}
		return bytes.NewReader(data), 0
	}
	return nil, 0
}

// gitUserName returns the configured git user.name, or "" on any error.
// Used as a fallback when --by is not given and $DOCOPS_USER is unset.
func gitUserName() string {
	out, err := exec.Command("git", "config", "user.name").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// openEditor launches $EDITOR (if set) on path. Best-effort; no error is
// surfaced if the editor exits non-zero — the file has already been written.
func openEditor(path string) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		return
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

// stringSliceFlag is a flag.Value collecting repeated --section entries.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error {
	if v != "" {
		*s = append(*s, v)
	}
	return nil
}
