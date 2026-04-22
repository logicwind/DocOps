package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/logicwind/docops/internal/audit"
	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/index"
	"github.com/logicwind/docops/internal/loader"
	"github.com/logicwind/docops/internal/validator"
)

// cmdAudit implements `docops audit [--json] [--only <rule>] [--include-not-needed]`.
// Exit codes:
//
//	0  no error-severity findings (warnings do not break the build)
//	1  one or more error-severity findings
//	2  bootstrap error (no config, invalid repo, validation errors)
func cmdAudit(args []string) int {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "emit findings as JSON")
	only := fs.String("only", "", "emit findings from this rule only")
	includeNotNeeded := fs.Bool("include-not-needed", false, "include adr-coverage-review info findings")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops audit [--json] [--only <rule>] [--include-not-needed]")
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
		fmt.Fprintf(os.Stderr, "docops audit: %v\n", err)
		return 2
	}
	cfg, root, err := config.FindAndLoad(cwd)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "docops audit: no docops.yaml found in this directory or any parent — run `docops init` first")
			return 2
		}
		fmt.Fprintf(os.Stderr, "docops audit: %v\n", err)
		return 2
	}

	set, err := loader.Load(root, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops audit: %v\n", err)
		return 2
	}

	// Refuse to audit a broken repo — same pattern as cmd_index.go.
	report := validator.Validate(set, cfg)
	if !report.OK() {
		fmt.Fprintf(os.Stderr, "docops audit: refusing to audit: %d validation error(s); run 'docops validate' to see them\n", len(report.Errors))
		return 2
	}

	idx, err := index.Build(set, cfg, root, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops audit: build index: %v\n", err)
		return 2
	}

	result := audit.Audit(idx, set, cfg, *includeNotNeeded)

	if *only != "" {
		result = result.FilterByRule(*only)
	}

	if *asJSON {
		b, err := result.JSON()
		if err != nil {
			fmt.Fprintf(os.Stderr, "docops audit: encode: %v\n", err)
			return 2
		}
		// MarshalIndent does not append a newline; add one for clean shell output.
		os.Stdout.Write(b)
		fmt.Println()
	} else {
		fmt.Print(result.Human())
	}

	if result.HasErrors() {
		return 1
	}
	return 0
}

// jsonAuditOutput mirrors the JSON shape for tests that need to decode it.
type jsonAuditOutput struct {
	OK       bool            `json:"ok"`
	Findings []audit.Finding `json:"findings"`
}

// decodeAuditJSON is a helper used by tests.
func decodeAuditJSON(b []byte) (jsonAuditOutput, error) {
	var out jsonAuditOutput
	if err := json.Unmarshal(b, &out); err != nil {
		return out, err
	}
	return out, nil
}
