package state

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nachiket/docops/internal/config"
	"github.com/nachiket/docops/internal/index"
)

// fixedNow is the reference clock used throughout these tests so no test
// reads the live wall clock.
var fixedNow = time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)

// defaultCfg returns the project defaults with all thresholds at their
// canonical values so test cases are not coupled to hard-coded numbers.
func defaultCfg() config.Config { return config.Default() }

// makeIndex builds a minimal *index.Index from a slice of IndexedDoc values.
// GeneratedAt and Version are filled in automatically.
func makeIndex(docs []index.IndexedDoc) *index.Index {
	return &index.Index{
		GeneratedAt: fixedNow.UTC().Format(time.RFC3339),
		Version:     index.IndexVersion,
		Docs:        docs,
	}
}

// rfc3339 converts a time.Time to RFC3339 — used when populating LastTouched.
func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// ---- Counts ----------------------------------------------------------------

func TestCounts_AllStatusValues(t *testing.T) {
	cfg := defaultCfg()

	docs := []index.IndexedDoc{
		// CTX: 2 active, 1 superseded
		{ID: "CTX-001", Kind: "CTX", LastTouched: rfc3339(fixedNow)},
		{ID: "CTX-002", Kind: "CTX", LastTouched: rfc3339(fixedNow)},
		{ID: "CTX-003", Kind: "CTX", SupersededBy: []string{"CTX-999"}, LastTouched: rfc3339(fixedNow)},

		// ADR: accepted×2, draft×1, superseded×1; coverage required×2, not-needed×1
		{ID: "ADR-0001", Kind: "ADR", ADRStatus: "accepted", ADRCoverage: "required", LastTouched: rfc3339(fixedNow)},
		{ID: "ADR-0002", Kind: "ADR", ADRStatus: "accepted", ADRCoverage: "not-needed", LastTouched: rfc3339(fixedNow)},
		{ID: "ADR-0003", Kind: "ADR", ADRStatus: "draft", ADRCoverage: "required", LastTouched: rfc3339(fixedNow)},
		{ID: "ADR-0004", Kind: "ADR", ADRStatus: "superseded", LastTouched: rfc3339(fixedNow)},

		// TP: backlog×1, active×2, blocked×1, done×3
		{ID: "TP-001", Kind: "TP", TaskStatus: "backlog", LastTouched: rfc3339(fixedNow)},
		{ID: "TP-002", Kind: "TP", TaskStatus: "active", LastTouched: rfc3339(fixedNow)},
		{ID: "TP-003", Kind: "TP", TaskStatus: "active", LastTouched: rfc3339(fixedNow)},
		{ID: "TP-004", Kind: "TP", TaskStatus: "blocked", LastTouched: rfc3339(fixedNow)},
		{ID: "TP-005", Kind: "TP", TaskStatus: "done", LastTouched: rfc3339(fixedNow)},
		{ID: "TP-006", Kind: "TP", TaskStatus: "done", LastTouched: rfc3339(fixedNow)},
		{ID: "TP-007", Kind: "TP", TaskStatus: "done", LastTouched: rfc3339(fixedNow)},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	c := snap.Counts

	if c.ContextActive != 2 {
		t.Errorf("ContextActive: want 2, got %d", c.ContextActive)
	}
	if c.ContextSuperseded != 1 {
		t.Errorf("ContextSuperseded: want 1, got %d", c.ContextSuperseded)
	}
	if c.ADRAccepted != 2 {
		t.Errorf("ADRAccepted: want 2, got %d", c.ADRAccepted)
	}
	if c.ADRDraft != 1 {
		t.Errorf("ADRDraft: want 1, got %d", c.ADRDraft)
	}
	if c.ADRSuperseded != 1 {
		t.Errorf("ADRSuperseded: want 1, got %d", c.ADRSuperseded)
	}
	if c.ADRCoverageRequired != 2 {
		t.Errorf("ADRCoverageRequired: want 2, got %d", c.ADRCoverageRequired)
	}
	if c.ADRCoverageNotNeeded != 1 {
		t.Errorf("ADRCoverageNotNeeded: want 1, got %d", c.ADRCoverageNotNeeded)
	}
	if c.TaskBacklog != 1 {
		t.Errorf("TaskBacklog: want 1, got %d", c.TaskBacklog)
	}
	if c.TaskActive != 2 {
		t.Errorf("TaskActive: want 2, got %d", c.TaskActive)
	}
	if c.TaskBlocked != 1 {
		t.Errorf("TaskBlocked: want 1, got %d", c.TaskBlocked)
	}
	if c.TaskDone != 3 {
		t.Errorf("TaskDone: want 3, got %d", c.TaskDone)
	}
}

// ---- Needs-attention rules -------------------------------------------------

// Rule 1a: accepted ADR + coverage:required + zero citing tasks + age > threshold → fires.
func TestAttention_AdrAcceptedNoCitingTasksOverThreshold(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.AdrAcceptedNoTasksAfterDays

	docs := []index.IndexedDoc{
		{
			ID:          "ADR-0001",
			Kind:        "ADR",
			ADRStatus:   "accepted",
			ADRCoverage: "required",
			AgeDays:     threshold + 1,
			LastTouched: rfc3339(fixedNow),
		},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	if len(snap.NeedsAttention) != 1 {
		t.Fatalf("want 1 attention bullet, got %d", len(snap.NeedsAttention))
	}
	if snap.NeedsAttention[0].ID != "ADR-0001" {
		t.Errorf("unexpected ID: %s", snap.NeedsAttention[0].ID)
	}
}

// Rule 1b: same ADR one day under threshold → no bullet.
func TestAttention_AdrAcceptedNoCitingTasksUnderThreshold(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.AdrAcceptedNoTasksAfterDays

	docs := []index.IndexedDoc{
		{
			ID:          "ADR-0001",
			Kind:        "ADR",
			ADRStatus:   "accepted",
			ADRCoverage: "required",
			AgeDays:     threshold, // exactly at threshold — should NOT fire (strictly greater)
			LastTouched: rfc3339(fixedNow),
		},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	if len(snap.NeedsAttention) != 0 {
		t.Errorf("want no attention bullets, got %d", len(snap.NeedsAttention))
	}
}

// Rule 1c: ADR with citing task does not fire even when over age threshold.
func TestAttention_AdrAcceptedWithCitingTask_NoFire(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.AdrAcceptedNoTasksAfterDays

	docs := []index.IndexedDoc{
		{
			ID:          "ADR-0001",
			Kind:        "ADR",
			ADRStatus:   "accepted",
			ADRCoverage: "required",
			AgeDays:     threshold + 10,
			LastTouched: rfc3339(fixedNow),
		},
		{
			ID:           "TP-001",
			Kind:         "TP",
			TaskStatus:   "active",
			TaskRequires: []string{"ADR-0001"},
			LastTouched:  rfc3339(fixedNow),
		},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	for _, a := range snap.NeedsAttention {
		if a.ID == "ADR-0001" {
			t.Errorf("ADR-0001 should not fire when a citing task exists")
		}
	}
}

// Rule 2a: draft ADR past threshold fires.
func TestAttention_AdrDraftOverThreshold(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.AdrDraftStaleDays

	docs := []index.IndexedDoc{
		{
			ID:          "ADR-0001",
			Kind:        "ADR",
			ADRStatus:   "draft",
			ADRCoverage: "required",
			AgeDays:     threshold + 1,
			LastTouched: rfc3339(fixedNow),
		},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	found := false
	for _, a := range snap.NeedsAttention {
		if a.ID == "ADR-0001" {
			found = true
			if a.Severity != "warn" {
				t.Errorf("want severity=warn, got %s", a.Severity)
			}
		}
	}
	if !found {
		t.Error("expected attention bullet for stale draft ADR")
	}
}

// Rule 2b: draft ADR just below threshold → no bullet.
func TestAttention_AdrDraftUnderThreshold(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.AdrDraftStaleDays

	docs := []index.IndexedDoc{
		{
			ID:          "ADR-0001",
			Kind:        "ADR",
			ADRStatus:   "draft",
			ADRCoverage: "required",
			AgeDays:     threshold,
			LastTouched: rfc3339(fixedNow),
		},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	for _, a := range snap.NeedsAttention {
		if a.ID == "ADR-0001" {
			t.Errorf("ADR-0001 draft should not fire at exactly the threshold")
		}
	}
}

// Rule 3a: active task past TaskActiveNoCommitsDays threshold fires.
func TestAttention_TaskActiveOverThreshold(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.TaskActiveNoCommitsDays

	docs := []index.IndexedDoc{
		{
			ID:          "TP-001",
			Kind:        "TP",
			TaskStatus:  "active",
			AgeDays:     threshold + 1,
			LastTouched: rfc3339(fixedNow),
		},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	found := false
	for _, a := range snap.NeedsAttention {
		if a.ID == "TP-001" {
			found = true
		}
	}
	if !found {
		t.Error("expected attention bullet for stalled active task")
	}
}

// Rule 3b: active task below threshold → no bullet.
func TestAttention_TaskActiveUnderThreshold(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.TaskActiveNoCommitsDays

	docs := []index.IndexedDoc{
		{
			ID:          "TP-001",
			Kind:        "TP",
			TaskStatus:  "active",
			AgeDays:     threshold,
			LastTouched: rfc3339(fixedNow),
		},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	for _, a := range snap.NeedsAttention {
		if a.ID == "TP-001" {
			t.Error("TP-001 should not fire at exactly the threshold")
		}
	}
}

// Rule 4a: task citing a superseded ADR fires with the configured severity.
func TestAttention_TaskCitesSupersededAdr_WarnSeverity(t *testing.T) {
	cfg := defaultCfg()
	cfg.Gaps.TaskRequiresSupersededAdr = config.SeverityWarn

	docs := []index.IndexedDoc{
		{ID: "ADR-0001", Kind: "ADR", ADRStatus: "superseded", LastTouched: rfc3339(fixedNow)},
		{
			ID:           "TP-001",
			Kind:         "TP",
			TaskStatus:   "active",
			TaskRequires: []string{"ADR-0001"},
			LastTouched:  rfc3339(fixedNow),
		},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	found := false
	for _, a := range snap.NeedsAttention {
		if a.ID == "TP-001" {
			found = true
			if a.Severity != "warn" {
				t.Errorf("want severity=warn, got %s", a.Severity)
			}
		}
	}
	if !found {
		t.Error("expected attention bullet for task citing superseded ADR")
	}
}

// Rule 4b: same scenario but severity = error.
func TestAttention_TaskCitesSupersededAdr_ErrorSeverity(t *testing.T) {
	cfg := defaultCfg()
	cfg.Gaps.TaskRequiresSupersededAdr = config.SeverityError

	docs := []index.IndexedDoc{
		{ID: "ADR-0001", Kind: "ADR", ADRStatus: "superseded", LastTouched: rfc3339(fixedNow)},
		{
			ID:           "TP-001",
			Kind:         "TP",
			TaskStatus:   "active",
			TaskRequires: []string{"ADR-0001"},
			LastTouched:  rfc3339(fixedNow),
		},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	found := false
	for _, a := range snap.NeedsAttention {
		if a.ID == "TP-001" {
			found = true
			if a.Severity != "error" {
				t.Errorf("want severity=error, got %s", a.Severity)
			}
		}
	}
	if !found {
		t.Error("expected attention bullet for task citing superseded ADR")
	}
}

// Rule 4c: same scenario but severity = off → no bullet.
func TestAttention_TaskCitesSupersededAdr_Off(t *testing.T) {
	cfg := defaultCfg()
	cfg.Gaps.TaskRequiresSupersededAdr = config.SeverityOff

	docs := []index.IndexedDoc{
		{ID: "ADR-0001", Kind: "ADR", ADRStatus: "superseded", LastTouched: rfc3339(fixedNow)},
		{
			ID:           "TP-001",
			Kind:         "TP",
			TaskStatus:   "active",
			TaskRequires: []string{"ADR-0001"},
			LastTouched:  rfc3339(fixedNow),
		},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	for _, a := range snap.NeedsAttention {
		if a.ID == "TP-001" {
			t.Error("TP-001 should not fire when superseded-ADR severity is off")
		}
	}
}

// Rule 4d: task citing a superseded CTX fires with configured severity.
func TestAttention_TaskCitesSupersededCtx_WarnSeverity(t *testing.T) {
	cfg := defaultCfg()
	cfg.Gaps.TaskRequiresSupersededCtx = config.SeverityWarn

	docs := []index.IndexedDoc{
		{ID: "CTX-001", Kind: "CTX", SupersededBy: []string{"CTX-002"}, LastTouched: rfc3339(fixedNow)},
		{
			ID:           "TP-001",
			Kind:         "TP",
			TaskStatus:   "active",
			TaskRequires: []string{"CTX-001"},
			LastTouched:  rfc3339(fixedNow),
		},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	found := false
	for _, a := range snap.NeedsAttention {
		if a.ID == "TP-001" {
			found = true
			if a.Severity != "warn" {
				t.Errorf("want severity=warn, got %s", a.Severity)
			}
		}
	}
	if !found {
		t.Error("expected attention bullet for task citing superseded CTX")
	}
}

// Rule 4e: severity off for superseded CTX suppresses bullet.
func TestAttention_TaskCitesSupersededCtx_Off(t *testing.T) {
	cfg := defaultCfg()
	cfg.Gaps.TaskRequiresSupersededCtx = config.SeverityOff

	docs := []index.IndexedDoc{
		{ID: "CTX-001", Kind: "CTX", SupersededBy: []string{"CTX-002"}, LastTouched: rfc3339(fixedNow)},
		{
			ID:           "TP-001",
			Kind:         "TP",
			TaskStatus:   "active",
			TaskRequires: []string{"CTX-001"},
			LastTouched:  rfc3339(fixedNow),
		},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	for _, a := range snap.NeedsAttention {
		if a.ID == "TP-001" {
			t.Error("TP-001 should not fire when superseded-CTX severity is off")
		}
	}
}

// Rule 5a: orphan CTX past threshold fires.
func TestAttention_OrphanCtxOverThreshold(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.CtxWithNoDerivedLinksAfterDays

	docs := []index.IndexedDoc{
		{
			ID:          "CTX-001",
			Kind:        "CTX",
			AgeDays:     threshold + 1,
			LastTouched: rfc3339(fixedNow),
			// no DerivedADRs, no ReferencedBy
		},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	found := false
	for _, a := range snap.NeedsAttention {
		if a.ID == "CTX-001" {
			found = true
			if a.Severity != "info" {
				t.Errorf("want severity=info, got %s", a.Severity)
			}
		}
	}
	if !found {
		t.Error("expected attention bullet for orphan CTX")
	}
}

// Rule 5b: CTX with DerivedADRs does not fire even when over age threshold.
func TestAttention_NonOrphanCtx_NoFire(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.CtxWithNoDerivedLinksAfterDays

	docs := []index.IndexedDoc{
		{
			ID:          "CTX-001",
			Kind:        "CTX",
			AgeDays:     threshold + 10,
			DerivedADRs: []string{"ADR-0001"},
			LastTouched: rfc3339(fixedNow),
		},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	for _, a := range snap.NeedsAttention {
		if a.ID == "CTX-001" {
			t.Error("CTX-001 should not fire when it has derived ADRs")
		}
	}
}

// Rule 5c: CTX with ReferencedBy does not fire.
func TestAttention_CtxWithReferencedBy_NoFire(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.CtxWithNoDerivedLinksAfterDays

	docs := []index.IndexedDoc{
		{
			ID:           "CTX-001",
			Kind:         "CTX",
			AgeDays:      threshold + 10,
			ReferencedBy: []index.Reference{{ID: "ADR-0001", Edge: "related"}},
			LastTouched:  rfc3339(fixedNow),
		},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	for _, a := range snap.NeedsAttention {
		if a.ID == "CTX-001" {
			t.Error("CTX-001 should not fire when it has ReferencedBy entries")
		}
	}
}

// Severity sort: error before warn before info.
func TestAttention_SeveritySort(t *testing.T) {
	cfg := defaultCfg()
	cfg.Gaps.TaskRequiresSupersededAdr = config.SeverityError
	cfg.Gaps.TaskRequiresSupersededCtx = config.SeverityWarn
	threshold := cfg.Gaps.CtxWithNoDerivedLinksAfterDays

	docs := []index.IndexedDoc{
		// info bullet
		{ID: "CTX-001", Kind: "CTX", AgeDays: threshold + 1, LastTouched: rfc3339(fixedNow)},
		// superseded ADR referenced by TP-001 → error
		{ID: "ADR-0001", Kind: "ADR", ADRStatus: "superseded", LastTouched: rfc3339(fixedNow)},
		{ID: "TP-001", Kind: "TP", TaskStatus: "active", TaskRequires: []string{"ADR-0001"}, LastTouched: rfc3339(fixedNow)},
		// superseded CTX referenced by TP-002 → warn
		{ID: "CTX-002", Kind: "CTX", SupersededBy: []string{"CTX-003"}, LastTouched: rfc3339(fixedNow)},
		{ID: "CTX-003", Kind: "CTX", LastTouched: rfc3339(fixedNow)},
		{ID: "TP-002", Kind: "TP", TaskStatus: "active", TaskRequires: []string{"CTX-002"}, LastTouched: rfc3339(fixedNow)},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	if len(snap.NeedsAttention) < 3 {
		t.Fatalf("want at least 3 bullets, got %d", len(snap.NeedsAttention))
	}
	order := []int{rankOf("error"), rankOf("warn"), rankOf("info")}
	for i := 1; i < len(snap.NeedsAttention); i++ {
		prev := rankOf(snap.NeedsAttention[i-1].Severity)
		curr := rankOf(snap.NeedsAttention[i].Severity)
		_ = order
		if prev > curr {
			t.Errorf("attention[%d] severity rank %d > attention[%d] rank %d — not sorted error→warn→info",
				i-1, prev, i, curr)
		}
	}
}

// ---- Active work -----------------------------------------------------------

func TestActiveWork_SortedByID(t *testing.T) {
	cfg := defaultCfg()

	docs := []index.IndexedDoc{
		{ID: "TP-003", Kind: "TP", TaskStatus: "active", CTXTitle: "Gamma", LastTouched: rfc3339(fixedNow)},
		{ID: "TP-001", Kind: "TP", TaskStatus: "active", CTXTitle: "Alpha", LastTouched: rfc3339(fixedNow)},
		{ID: "TP-002", Kind: "TP", TaskStatus: "active", CTXTitle: "Beta", LastTouched: rfc3339(fixedNow)},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	if len(snap.ActiveWork) != 3 {
		t.Fatalf("want 3 active tasks, got %d", len(snap.ActiveWork))
	}
	if snap.ActiveWork[0].ID != "TP-001" || snap.ActiveWork[1].ID != "TP-002" || snap.ActiveWork[2].ID != "TP-003" {
		t.Errorf("unexpected sort order: %v", snap.ActiveWork)
	}
}

func TestActiveWork_MarkdownFormatting(t *testing.T) {
	cfg := defaultCfg()

	docs := []index.IndexedDoc{
		// both assignee + priority
		{ID: "TP-001", Kind: "TP", TaskStatus: "active", CTXTitle: "Alpha work",
			TaskAssignee: "alice", TaskPriority: "p1", LastTouched: rfc3339(fixedNow)},
		// only assignee
		{ID: "TP-002", Kind: "TP", TaskStatus: "active", CTXTitle: "Beta work",
			TaskAssignee: "bob", LastTouched: rfc3339(fixedNow)},
		// only priority
		{ID: "TP-003", Kind: "TP", TaskStatus: "active", CTXTitle: "Gamma work",
			TaskPriority: "p2", LastTouched: rfc3339(fixedNow)},
		// neither
		{ID: "TP-004", Kind: "TP", TaskStatus: "active", CTXTitle: "Delta work",
			LastTouched: rfc3339(fixedNow)},
		// with requires
		{ID: "TP-005", Kind: "TP", TaskStatus: "active", CTXTitle: "Epsilon work",
			TaskAssignee: "carol", TaskPriority: "p1", TaskRequires: []string{"ADR-0001", "CTX-001"},
			LastTouched: rfc3339(fixedNow)},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	md := snap.Markdown()

	// Both assignee and priority.
	if !strings.Contains(md, "- TP-001 (alice, p1) Alpha work") {
		t.Errorf("expected 'TP-001 (alice, p1) Alpha work' in markdown, got:\n%s", md)
	}
	// Only assignee.
	if !strings.Contains(md, "- TP-002 (bob) Beta work") {
		t.Errorf("expected 'TP-002 (bob) Beta work' in markdown, got:\n%s", md)
	}
	// Only priority.
	if !strings.Contains(md, "- TP-003 (p2) Gamma work") {
		t.Errorf("expected 'TP-003 (p2) Gamma work' in markdown, got:\n%s", md)
	}
	// Neither assignee nor priority.
	if !strings.Contains(md, "- TP-004 Delta work") {
		t.Errorf("expected 'TP-004 Delta work' in markdown, got:\n%s", md)
	}
	// With requires.
	if !strings.Contains(md, "— requires: ADR-0001, CTX-001") {
		t.Errorf("expected requires list in markdown, got:\n%s", md)
	}
}

// ---- Recent activity -------------------------------------------------------

// When gitActivity is provided, it wins over the index fallback.
func TestRecentActivity_GitWins(t *testing.T) {
	cfg := defaultCfg()

	gitActivity := []ActivityEntry{
		{Date: "2026-04-21", ShortSHA: "abc1234", Subject: "update ADR-0001", Source: "git"},
		{Date: "2026-04-20", ShortSHA: "def5678", Subject: "add CTX-002", Source: "git"},
	}
	// Index has a doc touched very recently — should be ignored because gitActivity is set.
	docs := []index.IndexedDoc{
		{ID: "CTX-001", Kind: "CTX", CTXTitle: "vision", LastTouched: rfc3339(fixedNow)},
	}

	snap := Compute(makeIndex(docs), cfg, gitActivity, fixedNow)

	if len(snap.RecentActivity) != 2 {
		t.Fatalf("want 2 activity entries, got %d", len(snap.RecentActivity))
	}
	if snap.RecentActivity[0].ShortSHA != "abc1234" {
		t.Errorf("want newest entry first (abc1234), got %s", snap.RecentActivity[0].ShortSHA)
	}
}

// Git entries are sorted newest-first.
func TestRecentActivity_GitSortedNewestFirst(t *testing.T) {
	cfg := defaultCfg()

	gitActivity := []ActivityEntry{
		{Date: "2026-04-18", ShortSHA: "aaa0001", Subject: "old commit", Source: "git"},
		{Date: "2026-04-22", ShortSHA: "zzz9999", Subject: "newest commit", Source: "git"},
		{Date: "2026-04-20", ShortSHA: "bbb1234", Subject: "middle commit", Source: "git"},
	}

	snap := Compute(makeIndex(nil), cfg, gitActivity, fixedNow)

	if snap.RecentActivity[0].Date != "2026-04-22" {
		t.Errorf("want newest first (2026-04-22), got %s", snap.RecentActivity[0].Date)
	}
	if snap.RecentActivity[2].Date != "2026-04-18" {
		t.Errorf("want oldest last (2026-04-18), got %s", snap.RecentActivity[2].Date)
	}
}

// SHA tie-break: same date, higher SHA sorts first.
func TestRecentActivity_GitShaTieBreak(t *testing.T) {
	cfg := defaultCfg()

	gitActivity := []ActivityEntry{
		{Date: "2026-04-22", ShortSHA: "aaa1111", Subject: "commit A", Source: "git"},
		{Date: "2026-04-22", ShortSHA: "zzz9999", Subject: "commit Z", Source: "git"},
	}

	snap := Compute(makeIndex(nil), cfg, gitActivity, fixedNow)

	if snap.RecentActivity[0].ShortSHA != "zzz9999" {
		t.Errorf("want zzz9999 first (higher SHA), got %s", snap.RecentActivity[0].ShortSHA)
	}
}

// Git entries are capped at 20.
func TestRecentActivity_GitCapAt20(t *testing.T) {
	cfg := defaultCfg()

	var entries []ActivityEntry
	for i := 0; i < 25; i++ {
		entries = append(entries, ActivityEntry{
			Date:     "2026-04-22",
			ShortSHA: "sha0000" + string(rune('a'+i)),
			Subject:  "commit",
			Source:   "git",
		})
	}

	snap := Compute(makeIndex(nil), cfg, entries, fixedNow)
	if len(snap.RecentActivity) != 20 {
		t.Errorf("want 20 entries (capped), got %d", len(snap.RecentActivity))
	}
}

// Fallback from index when gitActivity is empty.
func TestRecentActivity_IndexFallback_WithinWindow(t *testing.T) {
	cfg := defaultCfg()
	windowDays := cfg.RecentActivityWindowDays

	recentTime := fixedNow.AddDate(0, 0, -(windowDays - 1))
	oldTime := fixedNow.AddDate(0, 0, -(windowDays + 1))

	docs := []index.IndexedDoc{
		{ID: "CTX-001", Kind: "CTX", CTXTitle: "recent doc", LastTouched: rfc3339(recentTime)},
		{ID: "ADR-0001", Kind: "ADR", CTXTitle: "old adr", LastTouched: rfc3339(oldTime)},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)

	if len(snap.RecentActivity) != 1 {
		t.Fatalf("want 1 activity entry (only within window), got %d: %v", len(snap.RecentActivity), snap.RecentActivity)
	}
	if !strings.Contains(snap.RecentActivity[0].Subject, "CTX-001") {
		t.Errorf("want CTX-001 in subject, got %s", snap.RecentActivity[0].Subject)
	}
}

// Fallback entries older than the window are excluded.
func TestRecentActivity_IndexFallback_OldEntriesExcluded(t *testing.T) {
	cfg := defaultCfg()
	windowDays := cfg.RecentActivityWindowDays

	oldTime := fixedNow.AddDate(0, 0, -(windowDays + 5))

	docs := []index.IndexedDoc{
		{ID: "CTX-001", Kind: "CTX", CTXTitle: "very old doc", LastTouched: rfc3339(oldTime)},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	if len(snap.RecentActivity) != 0 {
		t.Errorf("want 0 entries (all outside window), got %d", len(snap.RecentActivity))
	}
}

// Source field on git entries.
func TestRecentActivity_SourceTag_Git(t *testing.T) {
	cfg := defaultCfg()

	gitActivity := []ActivityEntry{
		{Date: "2026-04-22", ShortSHA: "abc1234", Subject: "fix"},
	}

	snap := Compute(makeIndex(nil), cfg, gitActivity, fixedNow)
	if snap.RecentActivity[0].Source != "git" {
		t.Errorf("want source=git, got %s", snap.RecentActivity[0].Source)
	}
}

// Source field on fallback index entries.
func TestRecentActivity_SourceTag_Index(t *testing.T) {
	cfg := defaultCfg()

	docs := []index.IndexedDoc{
		{ID: "CTX-001", Kind: "CTX", CTXTitle: "recent doc", LastTouched: rfc3339(fixedNow)},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	if len(snap.RecentActivity) == 0 {
		t.Fatal("expected at least one activity entry from index fallback")
	}
	if snap.RecentActivity[0].Source != "index" {
		t.Errorf("want source=index, got %s", snap.RecentActivity[0].Source)
	}
}

// ---- Determinism -----------------------------------------------------------

func TestDeterminism_MarkdownByteIdentical(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.AdrAcceptedNoTasksAfterDays

	docs := []index.IndexedDoc{
		{ID: "CTX-001", Kind: "CTX", CTXTitle: "vision", LastTouched: rfc3339(fixedNow)},
		{ID: "ADR-0001", Kind: "ADR", ADRStatus: "accepted", ADRCoverage: "required",
			AgeDays: threshold + 1, LastTouched: rfc3339(fixedNow)},
		{ID: "TP-001", Kind: "TP", TaskStatus: "active", CTXTitle: "do stuff",
			TaskAssignee: "alice", TaskPriority: "p1", LastTouched: rfc3339(fixedNow)},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	md1 := snap.Markdown()
	md2 := snap.Markdown()

	if md1 != md2 {
		t.Error("two consecutive Markdown() calls produced different output")
	}
}

func TestDeterminism_ComputeIdentical(t *testing.T) {
	cfg := defaultCfg()

	docs := []index.IndexedDoc{
		{ID: "CTX-001", Kind: "CTX", CTXTitle: "vision", LastTouched: rfc3339(fixedNow)},
		{ID: "ADR-0001", Kind: "ADR", ADRStatus: "accepted", ADRCoverage: "required",
			AgeDays: cfg.Gaps.AdrAcceptedNoTasksAfterDays + 1, LastTouched: rfc3339(fixedNow)},
		{ID: "TP-001", Kind: "TP", TaskStatus: "active", CTXTitle: "do stuff", LastTouched: rfc3339(fixedNow)},
	}

	idx := makeIndex(docs)
	snap1 := Compute(idx, cfg, nil, fixedNow)
	snap2 := Compute(idx, cfg, nil, fixedNow)

	// GeneratedAt will be identical because we pass the same clock.
	if snap1.GeneratedAt != snap2.GeneratedAt {
		t.Errorf("GeneratedAt differs: %s vs %s", snap1.GeneratedAt, snap2.GeneratedAt)
	}
	if len(snap1.NeedsAttention) != len(snap2.NeedsAttention) {
		t.Errorf("NeedsAttention length differs: %d vs %d", len(snap1.NeedsAttention), len(snap2.NeedsAttention))
	}
	if len(snap1.ActiveWork) != len(snap2.ActiveWork) {
		t.Errorf("ActiveWork length differs: %d vs %d", len(snap1.ActiveWork), len(snap2.ActiveWork))
	}
}

// ---- JSON marshalling ------------------------------------------------------

func TestJSON_ValidJSON(t *testing.T) {
	cfg := defaultCfg()

	docs := []index.IndexedDoc{
		{ID: "ADR-0001", Kind: "ADR", ADRStatus: "accepted", ADRCoverage: "not-needed",
			LastTouched: rfc3339(fixedNow)},
		{ID: "TP-001", Kind: "TP", TaskStatus: "active", CTXTitle: "some task",
			LastTouched: rfc3339(fixedNow)},
	}

	snap := Compute(makeIndex(docs), cfg, nil, fixedNow)
	out, err := snap.JSON()
	if err != nil {
		t.Fatalf("JSON() returned error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Errorf("JSON output does not parse as JSON: %v\n%s", err, out)
	}

	// Spot-check required top-level keys.
	for _, key := range []string{"generated_at", "counts", "needs_attention", "active_work", "recent_activity"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("JSON missing key %q", key)
		}
	}
}

// ---- formatMeta helper tests -----------------------------------------------

func TestFormatMeta_BothSet(t *testing.T) {
	if got := formatMeta("alice", "p1"); got != "alice, p1" {
		t.Errorf("want 'alice, p1', got %q", got)
	}
}

func TestFormatMeta_AssigneeOnly(t *testing.T) {
	if got := formatMeta("alice", ""); got != "alice" {
		t.Errorf("want 'alice', got %q", got)
	}
}

func TestFormatMeta_PriorityOnly(t *testing.T) {
	if got := formatMeta("", "p2"); got != "p2" {
		t.Errorf("want 'p2', got %q", got)
	}
}

func TestFormatMeta_Neither(t *testing.T) {
	if got := formatMeta("", ""); got != "" {
		t.Errorf("want empty string, got %q", got)
	}
}

// ---- Markdown structure tests ----------------------------------------------

func TestMarkdown_ContainsAllSections(t *testing.T) {
	snap := Compute(makeIndex(nil), defaultCfg(), nil, fixedNow)
	md := snap.Markdown()

	for _, section := range []string{
		"## Counts",
		"## Needs attention",
		"## Active work",
		"## Recent activity",
	} {
		if !strings.Contains(md, section) {
			t.Errorf("markdown missing section %q", section)
		}
	}
}

func TestMarkdown_EmptySnapshot(t *testing.T) {
	snap := Compute(makeIndex(nil), defaultCfg(), nil, fixedNow)
	md := snap.Markdown()

	if !strings.Contains(md, "- All clear.") {
		t.Error("empty snapshot should show 'All clear.' in needs-attention")
	}
	if !strings.Contains(md, "- (none)") {
		t.Error("empty snapshot should show '(none)' in active work or recent activity")
	}
}

func TestMarkdown_GeneratedAtDate(t *testing.T) {
	snap := Compute(makeIndex(nil), defaultCfg(), nil, fixedNow)
	md := snap.Markdown()

	if !strings.Contains(md, "2026-04-22") {
		t.Errorf("markdown should contain the date 2026-04-22, got:\n%s", md)
	}
}
