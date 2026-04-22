package audit

import (
	"fmt"
	"strings"

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/index"
	"github.com/logicwind/docops/internal/loader"
	"github.com/logicwind/docops/internal/schema"
)

// IndexedDoc.Kind values are the string forms of schema.Kind constants.
const (
	kindADR  = "ADR"
	kindTask = "TP"
	kindCTX  = "CTX"
)

// ruleADRAcceptedNoTasks fires when an ADR is accepted, coverage=required,
// has no citing tasks of any status, and is older than the threshold.
// "citing tasks" = any task whose requires[] includes this ADR ID regardless of status.
// We walk set.Order (all tasks) rather than relying on ActiveTasks which only
// surfaces active-status tasks.
func ruleADRAcceptedNoTasks(idx *index.Index, set *loader.DocSet, cfg config.Config) []Finding {
	citingCount := make(map[string]int)
	for _, id := range set.Order {
		doc := set.Docs[id]
		if doc.Kind != schema.KindTask {
			continue
		}
		for _, req := range doc.Requires() {
			citingCount[req]++
		}
	}

	threshold := cfg.Gaps.AdrAcceptedNoTasksAfterDays
	var out []Finding
	for _, d := range idx.Docs {
		if d.Kind != kindADR {
			continue
		}
		if d.ADRStatus != "accepted" || d.ADRCoverage != "required" {
			continue
		}
		if d.AgeDays <= threshold {
			continue
		}
		if citingCount[d.ID] > 0 {
			continue
		}
		out = append(out, Finding{
			Severity: "error",
			Rule:     "adr-accepted-no-tasks",
			ID:       d.ID,
			Path:     d.Path,
			Message:  fmt.Sprintf("ADR %s accepted with coverage=required but no citing tasks (age %d days, threshold %d)", d.ID, d.AgeDays, threshold),
			Action:   "create a citing task, or set coverage: not-needed with justification",
		})
	}
	return out
}

// ruleADRDraftStale fires when a draft ADR is older than the threshold.
func ruleADRDraftStale(idx *index.Index, cfg config.Config) []Finding {
	threshold := cfg.Gaps.AdrDraftStaleDays
	var out []Finding
	for _, d := range idx.Docs {
		if d.Kind != kindADR {
			continue
		}
		if d.ADRStatus != "draft" {
			continue
		}
		if d.AgeDays <= threshold {
			continue
		}
		out = append(out, Finding{
			Severity: "warn",
			Rule:     "adr-draft-stale",
			ID:       d.ID,
			Path:     d.Path,
			Message:  fmt.Sprintf("ADR %s has been draft for %d days (threshold %d)", d.ID, d.AgeDays, threshold),
			Action:   "accept or withdraw the draft",
		})
	}
	return out
}

// ruleTaskActiveStalled fires when an active task is older than the threshold.
func ruleTaskActiveStalled(idx *index.Index, cfg config.Config) []Finding {
	threshold := cfg.Gaps.TaskActiveNoCommitsDays
	var out []Finding
	for _, d := range idx.Docs {
		if d.Kind != kindTask {
			continue
		}
		if d.TaskStatus != "active" {
			continue
		}
		if d.AgeDays <= threshold {
			continue
		}
		out = append(out, Finding{
			Severity: "warn",
			Rule:     "task-active-stalled",
			ID:       d.ID,
			Path:     d.Path,
			Message:  fmt.Sprintf("task %s has been active for %d days without commits (threshold %d)", d.ID, d.AgeDays, threshold),
			Action:   "commit progress, block the task, or re-backlog",
		})
	}
	return out
}

// ruleTaskCitesSuperseded fires when a task's requires[] points to a superseded doc.
// Severity is configured via cfg.Gaps.TaskRequiresSupersededAdr / TaskRequiresSupersededCtx.
// If severity resolves to "off", the finding is omitted entirely.
func ruleTaskCitesSuperseded(idx *index.Index, set *loader.DocSet, cfg config.Config) []Finding {
	_ = set // set is accepted for interface symmetry; we use idx exclusively here

	byID := make(map[string]*index.IndexedDoc, len(idx.Docs))
	for i := range idx.Docs {
		d := &idx.Docs[i]
		byID[d.ID] = d
	}

	adrSev := string(cfg.Gaps.TaskRequiresSupersededAdr.Normalise(config.SeverityWarn))
	ctxSev := string(cfg.Gaps.TaskRequiresSupersededCtx.Normalise(config.SeverityWarn))

	var out []Finding
	for _, d := range idx.Docs {
		if d.Kind != kindTask {
			continue
		}
		for _, req := range d.TaskRequires {
			target, ok := byID[req]
			if !ok {
				continue
			}
			var sev, docKind string
			switch target.Kind {
			case kindADR:
				if target.ADRStatus != "superseded" {
					continue
				}
				sev = adrSev
				docKind = "ADR"
			case kindCTX:
				// A CTX is superseded when another doc declares it in supersedes[],
				// which populates SupersededBy in the index.
				if len(target.SupersededBy) == 0 {
					continue
				}
				sev = ctxSev
				docKind = "CTX"
			default:
				continue
			}
			if sev == "off" {
				continue
			}
			successor := ""
			if len(target.SupersededBy) > 0 {
				successor = "; succeeded by " + strings.Join(target.SupersededBy, ", ")
			}
			out = append(out, Finding{
				Severity: sev,
				Rule:     "task-cites-superseded",
				ID:       d.ID,
				Path:     d.Path,
				Message:  fmt.Sprintf("task %s requires superseded %s %s%s", d.ID, docKind, req, successor),
				Action:   "re-point to the successor",
			})
		}
	}
	return out
}

// ruleCtxOrphan fires when a CTX has no derived ADRs and no referenced_by entries
// and is older than the threshold.
func ruleCtxOrphan(idx *index.Index, cfg config.Config) []Finding {
	threshold := cfg.Gaps.CtxWithNoDerivedLinksAfterDays
	var out []Finding
	for _, d := range idx.Docs {
		if d.Kind != kindCTX {
			continue
		}
		if d.AgeDays <= threshold {
			continue
		}
		if len(d.DerivedADRs) > 0 || len(d.ReferencedBy) > 0 {
			continue
		}
		out = append(out, Finding{
			Severity: "warn",
			Rule:     "ctx-orphan",
			ID:       d.ID,
			Path:     d.Path,
			Message:  fmt.Sprintf("CTX %s has no derived ADRs or citations after %d days (threshold %d)", d.ID, d.AgeDays, threshold),
			Action:   "write an ADR from it or archive",
		})
	}
	return out
}

// ruleADRCoverageReview emits an info finding for every ADR with coverage=not-needed
// so teams can periodically revisit opt-outs.
func ruleADRCoverageReview(idx *index.Index) []Finding {
	var out []Finding
	for _, d := range idx.Docs {
		if d.Kind != kindADR {
			continue
		}
		if d.ADRCoverage != "not-needed" {
			continue
		}
		out = append(out, Finding{
			Severity: "info",
			Rule:     "adr-coverage-review",
			ID:       d.ID,
			Path:     d.Path,
			Message:  fmt.Sprintf("ADR %s has coverage=not-needed; periodically verify this is still accurate", d.ID),
			Action:   "review whether implementation tasks are still unnecessary",
		})
	}
	return out
}
