package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Human returns a human-readable multi-line report grouped by severity.
func (r *Report) Human() string {
	var buf bytes.Buffer

	errors := filterBySev(r.Findings, "error")
	warns := filterBySev(r.Findings, "warn")
	infos := filterBySev(r.Findings, "info")

	if len(errors) > 0 {
		fmt.Fprintln(&buf, "## Errors")
		for _, f := range errors {
			fmt.Fprintln(&buf, formatLine(f))
		}
	}
	if len(warns) > 0 {
		fmt.Fprintln(&buf, "## Warnings")
		for _, f := range warns {
			fmt.Fprintln(&buf, formatLine(f))
		}
	}
	if len(infos) > 0 {
		fmt.Fprintln(&buf, "## Info")
		for _, f := range infos {
			fmt.Fprintln(&buf, formatLine(f))
		}
	}

	fmt.Fprintf(&buf, "%d errors, %d warnings, %d info\n",
		len(errors), len(warns), len(infos))
	return buf.String()
}

func formatLine(f Finding) string {
	action := f.Action
	if action != "" {
		action = " -- " + action
	}
	loc := f.ID
	if f.Path != "" {
		loc = f.Path
	}
	_ = loc
	return fmt.Sprintf("[%s] %s %s: %s%s", f.Severity, f.Rule, f.ID, f.Message, action)
}

// JSON returns the structured JSON representation of the report.
func (r *Report) JSON() ([]byte, error) {
	findings := r.Findings
	if findings == nil {
		findings = []Finding{}
	}
	out := struct {
		OK       bool      `json:"ok"`
		Findings []Finding `json:"findings"`
	}{
		OK:       !r.HasErrors(),
		Findings: findings,
	}
	return json.MarshalIndent(out, "", "  ")
}

func filterBySev(fs []Finding, sev string) []Finding {
	var out []Finding
	for _, f := range fs {
		if f.Severity == sev {
			out = append(out, f)
		}
	}
	return out
}

// FilterByRule returns only findings matching the given rule name.
func (r *Report) FilterByRule(rule string) *Report {
	var out []Finding
	for _, f := range r.Findings {
		if strings.EqualFold(f.Rule, rule) {
			out = append(out, f)
		}
	}
	return &Report{Findings: out}
}
