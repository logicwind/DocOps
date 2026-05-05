// Package nextsteps renders the "→ Next:" affordance block printed
// after every mutating CLI command. The package owns the format; each
// command picks the suggestions appropriate to what it just did.
//
// Bar for a good Step: it is the *most likely* next move, fits on one
// line, and is a literal command the user can copy-paste. Never more
// than three; never punts to "see docs".
package nextsteps

import (
	"encoding/json"
	"fmt"
	"io"
)

// Step is one suggested follow-up — a short label plus the literal
// command (or slash-moment) the user can run.
type Step struct {
	Label   string `json:"label"`
	Command string `json:"command"`
}

// Render writes the standard "→ Next:" block to w. No-op if steps is
// empty. Callers are responsible for honoring --quiet / --json by not
// calling Render at all in those cases.
func Render(w io.Writer, steps []Step) {
	if len(steps) == 0 {
		return
	}
	fmt.Fprintln(w, "→ Next:")
	for _, s := range steps {
		fmt.Fprintf(w, "  • %s\n", s.Command)
		if s.Label != "" {
			fmt.Fprintf(w, "      %s\n", s.Label)
		}
	}
}

// EncodeJSON returns the steps in the shape used by --json command
// output: a "next_steps" key with an array of {label, command}. The
// caller merges this into its own JSON envelope.
func EncodeJSON(steps []Step) json.RawMessage {
	if len(steps) == 0 {
		return json.RawMessage("[]")
	}
	b, err := json.Marshal(steps)
	if err != nil {
		return json.RawMessage("[]")
	}
	return b
}
