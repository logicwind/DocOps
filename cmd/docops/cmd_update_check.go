package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/logicwind/docops/internal/updatecheck"
	"github.com/logicwind/docops/internal/version"
)

// cmdUpdateCheck implements `docops update-check`. Prints exactly one
// line (or zero, if the check was skipped) and always exits 0 — the
// subcommand is a probe, not a guard.
//
// Output:
//
//	UP_TO_DATE 0.1.2
//	UPGRADE_AVAILABLE 0.1.1 0.1.2
//	(nothing)                       — skipped/snoozed/disabled
//
// Flags:
//
//	--force    bypass cache (and snooze) for one fresh probe
//	--snooze   record a snooze for the current available remote
//	--json     emit a structured object instead of the line above
func cmdUpdateCheck(args []string) int {
	return runUpdateCheck(args, os.Stdout, os.Stderr, version.Version, "")
}

// runUpdateCheck is the testable core. local injects the version
// (otherwise read from the version package); stateDirOverride lets
// tests redirect the state dir without touching $HOME.
func runUpdateCheck(args []string, stdout, stderr io.Writer, local, stateDirOverride string) int {
	fs := flag.NewFlagSet("update-check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	force := fs.Bool("force", false, "bypass cache and snooze for one fresh probe")
	snooze := fs.Bool("snooze", false, "snooze the currently available upgrade")
	asJSON := fs.Bool("json", false, "emit a JSON object instead of the line format")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: docops update-check [--force] [--snooze] [--json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	stateDir := stateDirOverride
	if stateDir == "" {
		stateDir = updatecheck.DefaultStateDir()
	}

	res, err := updatecheck.Run(updatecheck.Opts{
		Local:     local,
		RemoteURL: os.Getenv("DOCOPS_REMOTE_URL"),
		StateDir:  stateDir,
		Force:     *force,
	})
	if err != nil {
		fmt.Fprintf(stderr, "docops update-check: %v\n", err)
		return 2
	}

	if *snooze && res.Status == updatecheck.StatusUpgradeAvailable {
		if err := updatecheck.Snooze(stateDir, res.Remote, time.Now()); err != nil {
			fmt.Fprintf(stderr, "docops update-check: snooze: %v\n", err)
			return 2
		}
		// After snoozing, future cached reads will suppress; for this
		// invocation, fall through to print the available upgrade so
		// the user sees what they just snoozed.
	}

	if *asJSON {
		emitUpdateCheckJSON(stdout, res)
		return 0
	}
	emitUpdateCheckLine(stdout, res)
	return 0
}

func emitUpdateCheckLine(w io.Writer, res updatecheck.Result) {
	switch res.Status {
	case updatecheck.StatusUpToDate:
		fmt.Fprintf(w, "UP_TO_DATE %s\n", res.Local)
	case updatecheck.StatusUpgradeAvailable:
		fmt.Fprintf(w, "UPGRADE_AVAILABLE %s %s\n", res.Local, res.Remote)
	case updatecheck.StatusSkipped:
		// Intentional: scripts treat silence as "no signal".
	}
}

func emitUpdateCheckJSON(w io.Writer, res updatecheck.Result) {
	payload := map[string]string{
		"status": res.Status.String(),
		"local":  res.Local,
	}
	if res.Remote != "" {
		payload["remote"] = res.Remote
	}
	if res.Reason != "" {
		payload["reason"] = res.Reason
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}
