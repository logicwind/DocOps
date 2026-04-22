// Package state computes the project snapshot emitted as docs/STATE.md.
// It consumes an *index.Index and config.Config; all side-effectful
// operations (git log, file writes) live in callers or in git.go.
package state

import (
	"fmt"
	"sort"
	"time"

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/index"
)

// Snapshot is the fully-computed project state. It can be marshalled
// directly to JSON or rendered as Markdown via Markdown().
type Snapshot struct {
	GeneratedAt    string          `json:"generated_at"` // RFC3339
	Counts         Counts          `json:"counts"`
	NeedsAttention []Attention     `json:"needs_attention"`
	ActiveWork     []ActiveTask    `json:"active_work"`
	RecentActivity []ActivityEntry `json:"recent_activity"`
}

// Counts holds per-kind document status tallies.
type Counts struct {
	ContextActive    int `json:"context_active"`
	ContextSuperseded int `json:"context_superseded"`

	ADRAccepted          int `json:"adr_accepted"`
	ADRDraft             int `json:"adr_draft"`
	ADRSuperseded        int `json:"adr_superseded"`
	ADRCoverageRequired  int `json:"adr_coverage_required"`
	ADRCoverageNotNeeded int `json:"adr_coverage_not_needed"`

	TaskBacklog int `json:"task_backlog"`
	TaskActive  int `json:"task_active"`
	TaskBlocked int `json:"task_blocked"`
	TaskDone    int `json:"task_done"`
}

// severityRank maps severity strings to sort weights (lower = higher priority).
var severityRank = map[string]int{"error": 0, "warn": 1, "info": 2}

func rankOf(sev string) int {
	if r, ok := severityRank[sev]; ok {
		return r
	}
	return 3
}

// Attention is a single "needs attention" bullet.
type Attention struct {
	ID       string `json:"id"`
	Severity string `json:"severity"` // "error" | "warn" | "info"
	Message  string `json:"message"`
}

// ActiveTask is one active task entry.
type ActiveTask struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Assignee string   `json:"assignee,omitempty"`
	Priority string   `json:"priority,omitempty"`
	Requires []string `json:"requires,omitempty"`
}

// ActivityEntry is one line from recent git/index activity.
type ActivityEntry struct {
	Date     string `json:"date"`         // YYYY-MM-DD
	ShortSHA string `json:"sha,omitempty"`
	Subject  string `json:"subject"`
	// Source indicates where the entry came from: "git" or "index"
	Source string `json:"source"`
}

// Compute derives a Snapshot from the index and config.
// gitActivity is the pre-fetched list from git.go (may be nil).
// now is the reference time for the header timestamp.
func Compute(idx *index.Index, cfg config.Config, gitActivity []ActivityEntry, now time.Time) *Snapshot {
	s := &Snapshot{
		GeneratedAt: now.UTC().Format(time.RFC3339),
	}

	// Build an ID→doc lookup for quick supersede checks.
	byID := make(map[string]index.IndexedDoc, len(idx.Docs))
	for _, d := range idx.Docs {
		byID[d.ID] = d
	}

	s.Counts = computeCounts(idx)
	s.NeedsAttention = computeAttention(idx, cfg, byID)
	s.ActiveWork = computeActiveWork(idx)
	s.RecentActivity = computeRecentActivity(gitActivity, idx, cfg, now)
	return s
}

func computeCounts(idx *index.Index) Counts {
	var c Counts
	for _, d := range idx.Docs {
		switch d.Kind {
		case "CTX":
			if len(d.SupersededBy) > 0 {
				c.ContextSuperseded++
			} else {
				c.ContextActive++
			}
		case "ADR":
			switch d.ADRStatus {
			case "accepted":
				c.ADRAccepted++
			case "draft":
				c.ADRDraft++
			case "superseded":
				c.ADRSuperseded++
			}
			switch d.ADRCoverage {
			case "required":
				c.ADRCoverageRequired++
			case "not-needed":
				c.ADRCoverageNotNeeded++
			}
		case "TP":
			switch d.TaskStatus {
			case "backlog":
				c.TaskBacklog++
			case "active":
				c.TaskActive++
			case "blocked":
				c.TaskBlocked++
			case "done":
				c.TaskDone++
			}
		}
	}
	return c
}

func computeAttention(idx *index.Index, cfg config.Config, byID map[string]index.IndexedDoc) []Attention {
	var bullets []Attention

	for _, d := range idx.Docs {
		switch d.Kind {
		case "ADR":
			// Rule 1: accepted + coverage:required + zero citing tasks + age > threshold.
			// ActiveTasks only covers active tasks; we scan all tasks for any citation.
			if d.ADRStatus == "accepted" &&
				d.ADRCoverage == "required" &&
				!hasAnyCitingTask(d, idx) &&
				d.AgeDays > cfg.Gaps.AdrAcceptedNoTasksAfterDays {
				bullets = append(bullets, Attention{
					ID:       d.ID,
					Severity: "warn",
					Message: fmt.Sprintf("%s accepted with no citing tasks after %d days — create a task or mark coverage: not-needed.",
						d.ID, cfg.Gaps.AdrAcceptedNoTasksAfterDays),
				})
			}
			// Rule 2: draft + age > threshold.
			if d.ADRStatus == "draft" && d.AgeDays > cfg.Gaps.AdrDraftStaleDays {
				bullets = append(bullets, Attention{
					ID:       d.ID,
					Severity: "warn",
					Message:  fmt.Sprintf("%s is a stale draft (%d days old) — accept, supersede, or delete.", d.ID, d.AgeDays),
				})
			}

		case "TP":
			// Rule 3: active + age > threshold (stalled with no recent commits).
			if d.TaskStatus == "active" && d.AgeDays > cfg.Gaps.TaskActiveNoCommitsDays {
				bullets = append(bullets, Attention{
					ID:       d.ID,
					Severity: "warn",
					Message:  fmt.Sprintf("%s has been active for %d days with no commits — unblock or close.", d.ID, d.AgeDays),
				})
			}
			// Rule 4: requires a superseded ADR or CTX.
			for _, req := range d.TaskRequires {
				ref, ok := byID[req]
				if !ok {
					continue
				}
				var sev config.Severity
				isSuperseded := false
				if ref.Kind == "ADR" && ref.ADRStatus == "superseded" {
					isSuperseded = true
					sev = cfg.Gaps.TaskRequiresSupersededAdr.Normalise(config.SeverityWarn)
				}
				if ref.Kind == "CTX" && len(ref.SupersededBy) > 0 {
					isSuperseded = true
					sev = cfg.Gaps.TaskRequiresSupersededCtx.Normalise(config.SeverityWarn)
				}
				if isSuperseded && sev != config.SeverityOff {
					bullets = append(bullets, Attention{
						ID:       d.ID,
						Severity: string(sev),
						Message:  fmt.Sprintf("%s cites superseded %s — update requires: or close the task.", d.ID, req),
					})
				}
			}

		case "CTX":
			// Rule 5: no derived ADRs + no referenced_by + age > threshold → potential orphan.
			if len(d.DerivedADRs) == 0 &&
				len(d.ReferencedBy) == 0 &&
				d.AgeDays > cfg.Gaps.CtxWithNoDerivedLinksAfterDays {
				bullets = append(bullets, Attention{
					ID:       d.ID,
					Severity: "info",
					Message: fmt.Sprintf("%s has no derived ADRs or references after %d days — potential orphan context.",
						d.ID, cfg.Gaps.CtxWithNoDerivedLinksAfterDays),
				})
			}
		}
	}

	// Sort by severity rank first, then by ID for determinism.
	sort.Slice(bullets, func(i, j int) bool {
		ri, rj := rankOf(bullets[i].Severity), rankOf(bullets[j].Severity)
		if ri != rj {
			return ri < rj
		}
		return bullets[i].ID < bullets[j].ID
	})
	return bullets
}

// hasAnyCitingTask checks whether any task (any status) in the index has this
// doc ID in its requires list. ActiveTasks only covers active tasks, so we
// must scan the full index here.
func hasAnyCitingTask(doc index.IndexedDoc, idx *index.Index) bool {
	for _, d := range idx.Docs {
		if d.Kind != "TP" {
			continue
		}
		for _, req := range d.TaskRequires {
			if req == doc.ID {
				return true
			}
		}
	}
	return false
}

func computeActiveWork(idx *index.Index) []ActiveTask {
	var tasks []ActiveTask
	for _, d := range idx.Docs {
		if d.Kind != "TP" || d.TaskStatus != "active" {
			continue
		}
		tasks = append(tasks, ActiveTask{
			ID:       d.ID,
			Title:    d.CTXTitle,
			Assignee: d.TaskAssignee,
			Priority: d.TaskPriority,
			Requires: d.TaskRequires,
		})
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	return tasks
}

func computeRecentActivity(gitActivity []ActivityEntry, idx *index.Index, cfg config.Config, now time.Time) []ActivityEntry {
	windowDays := cfg.RecentActivityWindowDays
	if windowDays <= 0 {
		windowDays = 7
	}
	since := now.AddDate(0, 0, -windowDays)

	if len(gitActivity) > 0 {
		var out []ActivityEntry
		for _, e := range gitActivity {
			if e.Source == "" {
				e.Source = "git"
			}
			out = append(out, e)
		}
		sortActivity(out)
		if len(out) > 20 {
			out = out[:20]
		}
		return out
	}

	// Fallback: derive activity from index last_touched when git is unavailable.
	var entries []ActivityEntry
	for _, d := range idx.Docs {
		t, err := time.Parse(time.RFC3339, d.LastTouched)
		if err != nil {
			continue
		}
		if t.Before(since) {
			continue
		}
		entries = append(entries, ActivityEntry{
			Date:    t.UTC().Format("2006-01-02"),
			Subject: d.ID + " — " + d.CTXTitle,
			Source:  "index",
		})
	}
	sortActivity(entries)
	if len(entries) > 20 {
		entries = entries[:20]
	}
	return entries
}

// sortActivity sorts entries newest-first, breaking ties by SHA.
func sortActivity(entries []ActivityEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Date != entries[j].Date {
			return entries[i].Date > entries[j].Date
		}
		return entries[i].ShortSHA > entries[j].ShortSHA
	})
}
