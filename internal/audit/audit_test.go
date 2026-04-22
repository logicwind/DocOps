package audit

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nachiket/docops/internal/config"
	"github.com/nachiket/docops/internal/index"
	"github.com/nachiket/docops/internal/loader"
	"github.com/nachiket/docops/internal/schema"
)

// defaultCfg returns a config with known thresholds suitable for unit tests.
func defaultCfg() config.Config {
	cfg := config.Default()
	cfg.Gaps.AdrAcceptedNoTasksAfterDays = 7
	cfg.Gaps.AdrDraftStaleDays = 14
	cfg.Gaps.TaskActiveNoCommitsDays = 5
	cfg.Gaps.CtxWithNoDerivedLinksAfterDays = 10
	cfg.Gaps.TaskRequiresSupersededAdr = config.SeverityWarn
	cfg.Gaps.TaskRequiresSupersededCtx = config.SeverityWarn
	return cfg
}

// makeIndex builds an *index.Index from a slice of IndexedDoc values.
func makeIndex(docs []index.IndexedDoc) *index.Index {
	return &index.Index{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Version:     index.IndexVersion,
		Docs:        docs,
	}
}

// makeDocSet builds a minimal *loader.DocSet from a slice of *loader.Doc values.
func makeDocSet(docs []*loader.Doc) *loader.DocSet {
	set := &loader.DocSet{
		Root: "/synthetic",
		Docs: make(map[string]*loader.Doc, len(docs)),
	}
	for _, d := range docs {
		set.Docs[d.ID] = d
		set.Order = append(set.Order, d.ID)
	}
	return set
}

// taskDoc builds a minimal *loader.Doc for a Task.
func taskLoaderDoc(id string, requires []string) *loader.Doc {
	return &loader.Doc{
		Kind: schema.KindTask,
		ID:   id,
		Path: "docs/tasks/" + id + "-x.md",
		Task: &schema.Task{
			Title:    id,
			Status:   "active",
			Requires: requires,
		},
	}
}

// ---- adr-accepted-no-tasks ----

// TestADRAcceptedNoTasks_Fires: accepted + coverage:required, no citing tasks, age > threshold → error.
func TestADRAcceptedNoTasks_Fires(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.AdrAcceptedNoTasksAfterDays

	idx := makeIndex([]index.IndexedDoc{
		{ID: "ADR-0001", Kind: kindADR, ADRStatus: "accepted", ADRCoverage: "required", AgeDays: threshold + 1},
	})
	set := makeDocSet(nil)

	r := Audit(idx, set, cfg, false)
	if !hasRule(r, "adr-accepted-no-tasks") {
		t.Errorf("expected adr-accepted-no-tasks finding; got: %+v", r.Findings)
	}
	if !r.HasErrors() {
		t.Error("expected HasErrors() true")
	}
}

// TestADRAcceptedNoTasks_SilentWithCitingTask: one citing task suppresses the finding.
func TestADRAcceptedNoTasks_SilentWithCitingTask(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.AdrAcceptedNoTasksAfterDays

	idx := makeIndex([]index.IndexedDoc{
		{ID: "ADR-0001", Kind: kindADR, ADRStatus: "accepted", ADRCoverage: "required", AgeDays: threshold + 1},
		{ID: "TP-001", Kind: kindTask, TaskStatus: "active", TaskRequires: []string{"ADR-0001"}, AgeDays: 1},
	})
	taskDoc := taskLoaderDoc("TP-001", []string{"ADR-0001"})
	set := makeDocSet([]*loader.Doc{taskDoc})

	r := Audit(idx, set, cfg, false)
	if hasRule(r, "adr-accepted-no-tasks") {
		t.Errorf("expected no adr-accepted-no-tasks finding when citing task exists; got: %+v", r.Findings)
	}
}

// TestADRAcceptedNoTasks_SilentJustBelowThreshold: age == threshold → silent.
func TestADRAcceptedNoTasks_SilentJustBelowThreshold(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.AdrAcceptedNoTasksAfterDays

	idx := makeIndex([]index.IndexedDoc{
		{ID: "ADR-0001", Kind: kindADR, ADRStatus: "accepted", ADRCoverage: "required", AgeDays: threshold},
	})
	set := makeDocSet(nil)

	r := Audit(idx, set, cfg, false)
	if hasRule(r, "adr-accepted-no-tasks") {
		t.Errorf("expected silence at threshold; got: %+v", r.Findings)
	}
}

// TestADRAcceptedNoTasks_SilentNotNeeded: coverage=not-needed suppresses the rule entirely.
func TestADRAcceptedNoTasks_SilentNotNeeded(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.AdrAcceptedNoTasksAfterDays

	idx := makeIndex([]index.IndexedDoc{
		{ID: "ADR-0001", Kind: kindADR, ADRStatus: "accepted", ADRCoverage: "not-needed", AgeDays: threshold + 100},
	})
	set := makeDocSet(nil)

	r := Audit(idx, set, cfg, false)
	if hasRule(r, "adr-accepted-no-tasks") {
		t.Errorf("coverage=not-needed should be silent; got: %+v", r.Findings)
	}
}

// ---- adr-draft-stale ----

func TestADRDraftStale_Fires(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.AdrDraftStaleDays

	idx := makeIndex([]index.IndexedDoc{
		{ID: "ADR-0001", Kind: kindADR, ADRStatus: "draft", AgeDays: threshold + 1},
	})
	r := Audit(idx, makeDocSet(nil), cfg, false)
	f := findingsForRule(r, "adr-draft-stale")
	if len(f) == 0 {
		t.Errorf("expected adr-draft-stale finding; got none")
	}
	if f[0].Severity != "warn" {
		t.Errorf("expected warn severity, got %q", f[0].Severity)
	}
}

func TestADRDraftStale_SilentJustBelowThreshold(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.AdrDraftStaleDays

	idx := makeIndex([]index.IndexedDoc{
		{ID: "ADR-0001", Kind: kindADR, ADRStatus: "draft", AgeDays: threshold},
	})
	r := Audit(idx, makeDocSet(nil), cfg, false)
	if hasRule(r, "adr-draft-stale") {
		t.Errorf("expected silence at threshold; got: %+v", r.Findings)
	}
}

// ---- task-active-stalled ----

func TestTaskActiveStalled_Fires(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.TaskActiveNoCommitsDays

	idx := makeIndex([]index.IndexedDoc{
		{ID: "TP-001", Kind: kindTask, TaskStatus: "active", AgeDays: threshold + 1},
	})
	r := Audit(idx, makeDocSet(nil), cfg, false)
	f := findingsForRule(r, "task-active-stalled")
	if len(f) == 0 {
		t.Errorf("expected task-active-stalled finding; got none")
	}
	if f[0].Severity != "warn" {
		t.Errorf("expected warn severity, got %q", f[0].Severity)
	}
}

func TestTaskActiveStalled_SilentJustBelowThreshold(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.TaskActiveNoCommitsDays

	idx := makeIndex([]index.IndexedDoc{
		{ID: "TP-001", Kind: kindTask, TaskStatus: "active", AgeDays: threshold},
	})
	r := Audit(idx, makeDocSet(nil), cfg, false)
	if hasRule(r, "task-active-stalled") {
		t.Errorf("expected silence at threshold; got: %+v", r.Findings)
	}
}

func TestTaskActiveStalled_SilentNonActiveTask(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.TaskActiveNoCommitsDays

	for _, status := range []string{"backlog", "blocked", "done"} {
		idx := makeIndex([]index.IndexedDoc{
			{ID: "TP-001", Kind: kindTask, TaskStatus: status, AgeDays: threshold + 100},
		})
		r := Audit(idx, makeDocSet(nil), cfg, false)
		if hasRule(r, "task-active-stalled") {
			t.Errorf("status=%q: expected silence; got: %+v", status, r.Findings)
		}
	}
}

// ---- task-cites-superseded ----

func TestTaskCitesSuperseded_ADRWarn(t *testing.T) {
	cfg := defaultCfg()
	cfg.Gaps.TaskRequiresSupersededAdr = config.SeverityWarn

	idx := makeIndex([]index.IndexedDoc{
		{ID: "ADR-0001", Kind: kindADR, ADRStatus: "superseded"},
		{ID: "TP-001", Kind: kindTask, TaskStatus: "active", TaskRequires: []string{"ADR-0001"}},
	})
	r := Audit(idx, makeDocSet(nil), cfg, false)
	f := findingsForRule(r, "task-cites-superseded")
	if len(f) == 0 {
		t.Errorf("expected task-cites-superseded finding; got none")
	}
	if f[0].Severity != "warn" {
		t.Errorf("expected warn, got %q", f[0].Severity)
	}
}

func TestTaskCitesSuperseded_ADRError(t *testing.T) {
	cfg := defaultCfg()
	cfg.Gaps.TaskRequiresSupersededAdr = config.SeverityError

	idx := makeIndex([]index.IndexedDoc{
		{ID: "ADR-0001", Kind: kindADR, ADRStatus: "superseded"},
		{ID: "TP-001", Kind: kindTask, TaskStatus: "active", TaskRequires: []string{"ADR-0001"}},
	})
	r := Audit(idx, makeDocSet(nil), cfg, false)
	f := findingsForRule(r, "task-cites-superseded")
	if len(f) == 0 {
		t.Errorf("expected task-cites-superseded finding; got none")
	}
	if f[0].Severity != "error" {
		t.Errorf("expected error, got %q", f[0].Severity)
	}
}

func TestTaskCitesSuperseded_ADROff(t *testing.T) {
	cfg := defaultCfg()
	cfg.Gaps.TaskRequiresSupersededAdr = config.SeverityOff

	idx := makeIndex([]index.IndexedDoc{
		{ID: "ADR-0001", Kind: kindADR, ADRStatus: "superseded"},
		{ID: "TP-001", Kind: kindTask, TaskStatus: "active", TaskRequires: []string{"ADR-0001"}},
	})
	r := Audit(idx, makeDocSet(nil), cfg, false)
	if hasRule(r, "task-cites-superseded") {
		t.Errorf("severity=off: expected silence; got: %+v", r.Findings)
	}
}

func TestTaskCitesSuperseded_CTXWarn(t *testing.T) {
	cfg := defaultCfg()
	cfg.Gaps.TaskRequiresSupersededCtx = config.SeverityWarn

	idx := makeIndex([]index.IndexedDoc{
		// A CTX is treated as superseded when SupersededBy is non-empty.
		{ID: "CTX-001", Kind: kindCTX, SupersededBy: []string{"CTX-002"}},
		{ID: "TP-001", Kind: kindTask, TaskStatus: "active", TaskRequires: []string{"CTX-001"}},
	})
	r := Audit(idx, makeDocSet(nil), cfg, false)
	f := findingsForRule(r, "task-cites-superseded")
	if len(f) == 0 {
		t.Errorf("expected finding for superseded CTX; got none")
	}
	if f[0].Severity != "warn" {
		t.Errorf("expected warn, got %q", f[0].Severity)
	}
}

// ---- ctx-orphan ----

func TestCtxOrphan_Fires(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.CtxWithNoDerivedLinksAfterDays

	idx := makeIndex([]index.IndexedDoc{
		{ID: "CTX-001", Kind: kindCTX, AgeDays: threshold + 1, DerivedADRs: nil, ReferencedBy: nil},
	})
	r := Audit(idx, makeDocSet(nil), cfg, false)
	f := findingsForRule(r, "ctx-orphan")
	if len(f) == 0 {
		t.Errorf("expected ctx-orphan finding; got none")
	}
	if f[0].Severity != "warn" {
		t.Errorf("expected warn, got %q", f[0].Severity)
	}
}

func TestCtxOrphan_SilentJustBelowThreshold(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.CtxWithNoDerivedLinksAfterDays

	idx := makeIndex([]index.IndexedDoc{
		{ID: "CTX-001", Kind: kindCTX, AgeDays: threshold},
	})
	r := Audit(idx, makeDocSet(nil), cfg, false)
	if hasRule(r, "ctx-orphan") {
		t.Errorf("expected silence at threshold; got: %+v", r.Findings)
	}
}

func TestCtxOrphan_SilentWithDerivedADR(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.CtxWithNoDerivedLinksAfterDays

	idx := makeIndex([]index.IndexedDoc{
		{ID: "CTX-001", Kind: kindCTX, AgeDays: threshold + 1, DerivedADRs: []string{"ADR-0001"}},
	})
	r := Audit(idx, makeDocSet(nil), cfg, false)
	if hasRule(r, "ctx-orphan") {
		t.Errorf("expected silence with derived ADR; got: %+v", r.Findings)
	}
}

func TestCtxOrphan_SilentWithReferencedBy(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.CtxWithNoDerivedLinksAfterDays

	idx := makeIndex([]index.IndexedDoc{
		{
			ID:           "CTX-001",
			Kind:         kindCTX,
			AgeDays:      threshold + 1,
			ReferencedBy: []index.Reference{{ID: "ADR-0001", Edge: "related"}},
		},
	})
	r := Audit(idx, makeDocSet(nil), cfg, false)
	if hasRule(r, "ctx-orphan") {
		t.Errorf("expected silence with referenced_by entry; got: %+v", r.Findings)
	}
}

// ---- adr-coverage-review ----

func TestADRCoverageReview_AbsentByDefault(t *testing.T) {
	cfg := defaultCfg()
	idx := makeIndex([]index.IndexedDoc{
		{ID: "ADR-0001", Kind: kindADR, ADRCoverage: "not-needed"},
	})
	r := Audit(idx, makeDocSet(nil), cfg, false)
	if hasRule(r, "adr-coverage-review") {
		t.Errorf("expected adr-coverage-review to be absent by default; got: %+v", r.Findings)
	}
}

func TestADRCoverageReview_PresentWhenIncludeNotNeeded(t *testing.T) {
	cfg := defaultCfg()
	idx := makeIndex([]index.IndexedDoc{
		{ID: "ADR-0001", Kind: kindADR, ADRCoverage: "not-needed"},
	})
	r := Audit(idx, makeDocSet(nil), cfg, true)
	f := findingsForRule(r, "adr-coverage-review")
	if len(f) == 0 {
		t.Errorf("expected adr-coverage-review finding with includeNotNeeded=true; got none")
	}
	if f[0].Severity != "info" {
		t.Errorf("expected info severity, got %q", f[0].Severity)
	}
}

func TestADRCoverageReview_OnlyNotNeeded(t *testing.T) {
	cfg := defaultCfg()
	idx := makeIndex([]index.IndexedDoc{
		{ID: "ADR-0001", Kind: kindADR, ADRCoverage: "required"},
		{ID: "ADR-0002", Kind: kindADR, ADRCoverage: "not-needed"},
	})
	r := Audit(idx, makeDocSet(nil), cfg, true)
	f := findingsForRule(r, "adr-coverage-review")
	if len(f) != 1 {
		t.Errorf("expected exactly 1 adr-coverage-review finding, got %d", len(f))
	}
	if len(f) > 0 && f[0].ID != "ADR-0002" {
		t.Errorf("expected finding for ADR-0002, got %q", f[0].ID)
	}
}

// ---- FilterByRule ----

func TestFilterByRule_Narrows(t *testing.T) {
	cfg := defaultCfg()
	threshold := cfg.Gaps.AdrDraftStaleDays

	idx := makeIndex([]index.IndexedDoc{
		{ID: "ADR-0001", Kind: kindADR, ADRStatus: "draft", AgeDays: threshold + 1},
		{ID: "TP-001", Kind: kindTask, TaskStatus: "active", AgeDays: cfg.Gaps.TaskActiveNoCommitsDays + 1},
	})
	r := Audit(idx, makeDocSet(nil), cfg, false)
	filtered := r.FilterByRule("adr-draft-stale")
	for _, f := range filtered.Findings {
		if f.Rule != "adr-draft-stale" {
			t.Errorf("FilterByRule left unexpected rule %q", f.Rule)
		}
	}
	if !hasRule(filtered, "adr-draft-stale") {
		t.Error("FilterByRule removed the target rule's finding")
	}
}

func TestFilterByRule_EmptyWhenNoMatch(t *testing.T) {
	cfg := defaultCfg()
	idx := makeIndex([]index.IndexedDoc{
		{ID: "ADR-0001", Kind: kindADR, ADRStatus: "draft", AgeDays: cfg.Gaps.AdrDraftStaleDays + 1},
	})
	r := Audit(idx, makeDocSet(nil), cfg, false)
	filtered := r.FilterByRule("ctx-orphan")
	if len(filtered.Findings) != 0 {
		t.Errorf("expected empty report when rule matches nothing; got %d findings", len(filtered.Findings))
	}
}

// ---- HasErrors ----

func TestHasErrors_TrueWithError(t *testing.T) {
	r := &Report{Findings: []Finding{{Severity: "error", Rule: "x", ID: "Y"}}}
	if !r.HasErrors() {
		t.Error("expected HasErrors() true with error finding")
	}
}

func TestHasErrors_FalseWithWarnOnly(t *testing.T) {
	r := &Report{Findings: []Finding{{Severity: "warn", Rule: "x", ID: "Y"}}}
	if r.HasErrors() {
		t.Error("expected HasErrors() false with only warn")
	}
}

func TestHasErrors_FalseWithInfoOnly(t *testing.T) {
	r := &Report{Findings: []Finding{{Severity: "info", Rule: "x", ID: "Y"}}}
	if r.HasErrors() {
		t.Error("expected HasErrors() false with only info")
	}
}

// ---- Determinism ----

func TestDeterminism_SortOrder(t *testing.T) {
	// Build a report with findings of all three severities; verify order.
	findings := []Finding{
		{Severity: "info", Rule: "adr-coverage-review", ID: "ADR-0003"},
		{Severity: "warn", Rule: "adr-draft-stale", ID: "ADR-0001"},
		{Severity: "error", Rule: "adr-accepted-no-tasks", ID: "ADR-0002"},
	}
	sortFindings(findings)

	if findings[0].Severity != "error" {
		t.Errorf("expected error first, got %q", findings[0].Severity)
	}
	if findings[1].Severity != "warn" {
		t.Errorf("expected warn second, got %q", findings[1].Severity)
	}
	if findings[2].Severity != "info" {
		t.Errorf("expected info third, got %q", findings[2].Severity)
	}

	// Two runs of Human() on the same report must be byte-identical.
	r := &Report{Findings: findings}
	h1 := r.Human()
	h2 := r.Human()
	if h1 != h2 {
		t.Errorf("Human() not deterministic:\n%s\nvs\n%s", h1, h2)
	}

	// Two runs of JSON() must also be byte-identical.
	j1, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	j2, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(j1) != string(j2) {
		t.Error("JSON() not deterministic")
	}

	// JSON must also be valid.
	var v interface{}
	if err := json.Unmarshal(j1, &v); err != nil {
		t.Errorf("JSON() produced invalid JSON: %v", err)
	}

	// Verify sort within same severity by rule, then id.
	multiSev := []Finding{
		{Severity: "warn", Rule: "ctx-orphan", ID: "CTX-002"},
		{Severity: "warn", Rule: "adr-draft-stale", ID: "ADR-0002"},
		{Severity: "warn", Rule: "adr-draft-stale", ID: "ADR-0001"},
	}
	sortFindings(multiSev)
	if multiSev[0].Rule != "adr-draft-stale" || multiSev[0].ID != "ADR-0001" {
		t.Errorf("unexpected order: %+v", multiSev)
	}
	if multiSev[1].Rule != "adr-draft-stale" || multiSev[1].ID != "ADR-0002" {
		t.Errorf("unexpected order: %+v", multiSev)
	}
	if multiSev[2].Rule != "ctx-orphan" {
		t.Errorf("unexpected order: %+v", multiSev)
	}
}

// ---- Human and JSON output ----

func TestHuman_NonEmpty(t *testing.T) {
	r := &Report{Findings: []Finding{
		{Severity: "warn", Rule: "adr-draft-stale", ID: "ADR-0001", Message: "stale"},
	}}
	out := r.Human()
	if !strings.Contains(out, "Warnings") {
		t.Errorf("expected 'Warnings' section in Human() output; got: %q", out)
	}
	if !strings.Contains(out, "ADR-0001") {
		t.Errorf("expected ADR-0001 in Human() output; got: %q", out)
	}
}

func TestJSON_Structure(t *testing.T) {
	r := &Report{Findings: []Finding{
		{Severity: "error", Rule: "adr-accepted-no-tasks", ID: "ADR-0001", Message: "no tasks"},
	}}
	b, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		OK       bool  `json:"ok"`
		Findings []interface{} `json:"findings"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if out.OK {
		t.Error("ok should be false when errors present")
	}
	if len(out.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(out.Findings))
	}
}

func TestJSON_EmptyFindings(t *testing.T) {
	r := &Report{}
	b, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"findings": []`) {
		t.Errorf("expected empty findings array, got: %s", string(b))
	}
}

// ---- helpers ----

func hasRule(r *Report, rule string) bool {
	return len(findingsForRule(r, rule)) > 0
}

func findingsForRule(r *Report, rule string) []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}
