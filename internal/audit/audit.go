// Package audit evaluates the enriched index against gap rules and
// returns a structured Report of actionable findings.
package audit

import (
	"sort"

	"github.com/nachiket/docops/internal/config"
	"github.com/nachiket/docops/internal/index"
	"github.com/nachiket/docops/internal/loader"
)

// Finding is one actionable gap detected by an audit rule.
type Finding struct {
	Severity string `json:"severity"` // error | warn | info
	Rule     string `json:"rule"`
	ID       string `json:"id"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
	Action   string `json:"action,omitempty"`
}

// Report is the result of Audit.
type Report struct {
	Findings []Finding
}

// HasErrors reports whether any error-severity finding is present.
func (r *Report) HasErrors() bool {
	for _, f := range r.Findings {
		if f.Severity == "error" {
			return true
		}
	}
	return false
}

// Audit runs all gap rules against the provided index and doc set.
// includeNotNeeded enables the adr-coverage-review info rule.
func Audit(idx *index.Index, set *loader.DocSet, cfg config.Config, includeNotNeeded bool) *Report {
	var all []Finding
	all = append(all, ruleADRAcceptedNoTasks(idx, set, cfg)...)
	all = append(all, ruleADRDraftStale(idx, cfg)...)
	all = append(all, ruleTaskActiveStalled(idx, cfg)...)
	all = append(all, ruleTaskCitesSuperseded(idx, set, cfg)...)
	all = append(all, ruleCtxOrphan(idx, cfg)...)
	if includeNotNeeded {
		all = append(all, ruleADRCoverageReview(idx)...)
	}
	sortFindings(all)
	return &Report{Findings: all}
}

// severityRank maps severity to a sort key (lower == earlier).
func severityRank(s string) int {
	switch s {
	case "error":
		return 0
	case "warn":
		return 1
	case "info":
		return 2
	default:
		return 3
	}
}

func sortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		ri, rj := severityRank(fs[i].Severity), severityRank(fs[j].Severity)
		if ri != rj {
			return ri < rj
		}
		if fs[i].Rule != fs[j].Rule {
			return fs[i].Rule < fs[j].Rule
		}
		return fs[i].ID < fs[j].ID
	})
}
