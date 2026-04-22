package index

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/loader"
)

// fixedNow is a reference time used in all tests so age_days is deterministic.
var fixedNow = time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)

// scenario builds a throwaway project with the given files relative to a temp
// root, loads the DocSet with defaults, and returns it along with the root.
func scenario(t *testing.T, files map[string]string) (*loader.DocSet, config.Config, string) {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Default()
	set, err := loader.Load(root, cfg)
	if err != nil {
		t.Fatalf("loader.Load: %v", err)
	}
	return set, cfg, root
}

// findDoc returns the IndexedDoc with the given ID, or fails the test.
func findDoc(t *testing.T, idx *Index, id string) IndexedDoc {
	t.Helper()
	for _, d := range idx.Docs {
		if d.ID == id {
			return d
		}
	}
	t.Fatalf("doc %q not found in index", id)
	return IndexedDoc{}
}

// ---- implementation derivation ----

func TestImplementation_NotNeeded(t *testing.T) {
	set, cfg, root := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-x.md": adrDoc("accepted", "not-needed"),
	})
	idx, err := Build(set, cfg, root, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	d := findDoc(t, idx, "ADR-0001")
	if d.Implementation != "n/a" {
		t.Errorf("want n/a, got %q", d.Implementation)
	}
}

func TestImplementation_NoCitingTasks(t *testing.T) {
	set, cfg, root := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-x.md": adrDoc("accepted", "required"),
	})
	idx, err := Build(set, cfg, root, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	d := findDoc(t, idx, "ADR-0001")
	if d.Implementation != "not-started" {
		t.Errorf("want not-started, got %q", d.Implementation)
	}
}

func TestImplementation_AllDone(t *testing.T) {
	set, cfg, root := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-x.md":  adrDoc("accepted", "required"),
		"docs/tasks/TP-001-work.md":      taskDoc("done", []string{"ADR-0001"}, nil),
	})
	idx, err := Build(set, cfg, root, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	d := findDoc(t, idx, "ADR-0001")
	if d.Implementation != "done" {
		t.Errorf("want done, got %q", d.Implementation)
	}
}

func TestImplementation_AnyActive(t *testing.T) {
	set, cfg, root := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-x.md":  adrDoc("accepted", "required"),
		"docs/tasks/TP-001-work.md":      taskDoc("active", []string{"ADR-0001"}, nil),
		"docs/tasks/TP-002-more.md":      taskDoc("done", []string{"ADR-0001"}, nil),
	})
	idx, err := Build(set, cfg, root, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	d := findDoc(t, idx, "ADR-0001")
	if d.Implementation != "in-progress" {
		t.Errorf("want in-progress, got %q", d.Implementation)
	}
}

func TestImplementation_Partial(t *testing.T) {
	set, cfg, root := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-x.md":  adrDoc("accepted", "required"),
		"docs/tasks/TP-001-work.md":      taskDoc("done", []string{"ADR-0001"}, nil),
		"docs/tasks/TP-002-more.md":      taskDoc("backlog", []string{"ADR-0001"}, nil),
	})
	idx, err := Build(set, cfg, root, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	d := findDoc(t, idx, "ADR-0001")
	if d.Implementation != "partial" {
		t.Errorf("want partial, got %q", d.Implementation)
	}
}

func TestImplementation_AllBacklog(t *testing.T) {
	set, cfg, root := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-x.md":  adrDoc("accepted", "required"),
		"docs/tasks/TP-001-work.md":      taskDoc("backlog", []string{"ADR-0001"}, nil),
	})
	idx, err := Build(set, cfg, root, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	d := findDoc(t, idx, "ADR-0001")
	if d.Implementation != "not-started" {
		t.Errorf("want not-started (all backlog), got %q", d.Implementation)
	}
}

// ---- reverse edge resolution ----

func TestReverseEdges_SupersededBy(t *testing.T) {
	set, cfg, root := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-old.md": adrDocSupersedes("superseded", "not-needed", nil),
		"docs/decisions/ADR-0002-new.md": adrDocSupersedes("accepted", "not-needed", []string{"ADR-0001"}),
	})
	idx, err := Build(set, cfg, root, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	old := findDoc(t, idx, "ADR-0001")
	if len(old.SupersededBy) != 1 || old.SupersededBy[0] != "ADR-0002" {
		t.Errorf("want SupersededBy=[ADR-0002], got %v", old.SupersededBy)
	}
}

func TestReverseEdges_ReferencedBy(t *testing.T) {
	set, cfg, root := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-x.md": adrDoc("accepted", "required"),
		"docs/tasks/TP-001-work.md":    taskDoc("backlog", []string{"ADR-0001"}, nil),
	})
	idx, err := Build(set, cfg, root, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	adr := findDoc(t, idx, "ADR-0001")
	if len(adr.ReferencedBy) != 1 {
		t.Fatalf("want 1 referenced_by entry, got %d", len(adr.ReferencedBy))
	}
	if adr.ReferencedBy[0].ID != "TP-001" || adr.ReferencedBy[0].Edge != "requires" {
		t.Errorf("unexpected referenced_by: %+v", adr.ReferencedBy[0])
	}
}

func TestReverseEdges_Blocks(t *testing.T) {
	set, cfg, root := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-x.md": adrDoc("accepted", "not-needed"),
		"docs/tasks/TP-001-first.md":   taskDoc("backlog", []string{"ADR-0001"}, nil),
		"docs/tasks/TP-002-second.md":  taskDoc("backlog", []string{"ADR-0001"}, []string{"TP-001"}),
	})
	idx, err := Build(set, cfg, root, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	first := findDoc(t, idx, "TP-001")
	if len(first.Blocks) != 1 || first.Blocks[0] != "TP-002" {
		t.Errorf("want Blocks=[TP-002], got %v", first.Blocks)
	}
}

func TestReverseEdges_ActiveTasks(t *testing.T) {
	set, cfg, root := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-x.md": adrDoc("accepted", "required"),
		"docs/tasks/TP-001-work.md":    taskDoc("active", []string{"ADR-0001"}, nil),
		"docs/tasks/TP-002-more.md":    taskDoc("backlog", []string{"ADR-0001"}, nil),
	})
	idx, err := Build(set, cfg, root, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	adr := findDoc(t, idx, "ADR-0001")
	if len(adr.ActiveTasks) != 1 || adr.ActiveTasks[0] != "TP-001" {
		t.Errorf("want ActiveTasks=[TP-001], got %v", adr.ActiveTasks)
	}
}

// ---- stale computation with synthetic clock ----

func TestStale_ADRDraftOld(t *testing.T) {
	set, cfg, root := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-x.md": adrDoc("draft", "required"),
	})
	// Use a now that's far in the future relative to mtime so age_days > threshold.
	future := fixedNow.Add(30 * 24 * time.Hour)
	idx, err := Build(set, cfg, root, future)
	if err != nil {
		t.Fatal(err)
	}
	d := findDoc(t, idx, "ADR-0001")
	if !d.Stale {
		t.Error("expected stale=true for old draft ADR")
	}
}

func TestStale_ADRDraftFresh(t *testing.T) {
	set, cfg, root := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-x.md": adrDoc("draft", "required"),
	})
	// now == mtime means age_days == 0, well under threshold.
	idx, err := Build(set, cfg, root, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	d := findDoc(t, idx, "ADR-0001")
	if d.Stale {
		t.Error("expected stale=false for fresh draft ADR")
	}
}

func TestStale_TaskActiveOld(t *testing.T) {
	set, cfg, root := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-x.md": adrDoc("accepted", "not-needed"),
		"docs/tasks/TP-001-work.md":    taskDoc("active", []string{"ADR-0001"}, nil),
	})
	future := fixedNow.Add(30 * 24 * time.Hour)
	idx, err := Build(set, cfg, root, future)
	if err != nil {
		t.Fatal(err)
	}
	d := findDoc(t, idx, "TP-001")
	if !d.Stale {
		t.Error("expected stale=true for old active task")
	}
}

func TestStale_CTXOrphan(t *testing.T) {
	set, cfg, root := scenario(t, map[string]string{
		"docs/context/CTX-001-v.md": ctxDoc("vision", "brief"),
	})
	future := fixedNow.Add(30 * 24 * time.Hour)
	idx, err := Build(set, cfg, root, future)
	if err != nil {
		t.Fatal(err)
	}
	d := findDoc(t, idx, "CTX-001")
	if !d.Stale {
		t.Error("expected stale=true for orphan CTX with no derived ADRs")
	}
}

// ---- body summary edge cases ----

func TestBodySummary_Empty(t *testing.T) {
	s, wc := bodySummary([]byte(""))
	if s != "" {
		t.Errorf("expected empty summary, got %q", s)
	}
	if wc != 0 {
		t.Errorf("expected 0 words, got %d", wc)
	}
}

func TestBodySummary_HeadingOnly(t *testing.T) {
	s, _ := bodySummary([]byte("# Heading\n\n## Sub\n"))
	if s != "" {
		t.Errorf("heading-only body should yield empty summary, got %q", s)
	}
}

func TestBodySummary_ExplicitOverride(t *testing.T) {
	body := []byte("# Title\n\n<!-- summary: my override -->\n\nsome paragraph here")
	s, _ := bodySummary(body)
	if s != "my override" {
		t.Errorf("expected override, got %q", s)
	}
}

func TestBodySummary_Truncation(t *testing.T) {
	// Build a body with a first paragraph longer than 200 chars.
	long := "word "
	for i := 0; i < 50; i++ {
		long += "word "
	}
	s, _ := bodySummary([]byte(long))
	// Should be capped and end with ellipsis.
	runes := []rune(s)
	if len(runes) > summaryMaxLen+1 { // +1 for ellipsis rune
		t.Errorf("summary too long: %d runes", len(runes))
	}
	if runes[len(runes)-1] != '…' {
		t.Errorf("truncated summary should end with ellipsis, got %q", string(runes[len(runes)-1]))
	}
}

func TestBodySummary_FirstParagraphSkipsHeadings(t *testing.T) {
	body := []byte("# Title\n\nFirst prose paragraph.\n\nSecond paragraph.\n")
	s, _ := bodySummary(body)
	if s != "First prose paragraph." {
		t.Errorf("expected first prose paragraph, got %q", s)
	}
}

// ---- determinism ----

func TestIndex_Deterministic(t *testing.T) {
	set, cfg, root := scenario(t, map[string]string{
		"docs/context/CTX-001-v.md":     ctxDoc("vision", "brief"),
		"docs/decisions/ADR-0001-x.md":  adrDoc("accepted", "required"),
		"docs/decisions/ADR-0002-y.md":  adrDoc("accepted", "not-needed"),
		"docs/tasks/TP-001-work.md":     taskDoc("backlog", []string{"ADR-0001"}, nil),
		"docs/tasks/TP-002-more.md":     taskDoc("active", []string{"ADR-0001", "ADR-0002"}, []string{"TP-001"}),
	})

	idx1, err := Build(set, cfg, root, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	idx2, err := Build(set, cfg, root, fixedNow)
	if err != nil {
		t.Fatal(err)
	}

	if len(idx1.Docs) != len(idx2.Docs) {
		t.Fatalf("doc counts differ: %d vs %d", len(idx1.Docs), len(idx2.Docs))
	}
	for i := range idx1.Docs {
		d1, d2 := idx1.Docs[i], idx2.Docs[i]
		if d1.ID != d2.ID {
			t.Errorf("index %d: ID %q vs %q", i, d1.ID, d2.ID)
		}
		if d1.Implementation != d2.Implementation {
			t.Errorf("index %d: Implementation %q vs %q", i, d1.Implementation, d2.Implementation)
		}
	}
}

func TestIndex_SortedByID(t *testing.T) {
	set, cfg, root := scenario(t, map[string]string{
		"docs/decisions/ADR-0002-y.md": adrDoc("accepted", "not-needed"),
		"docs/decisions/ADR-0001-x.md": adrDoc("accepted", "not-needed"),
		"docs/tasks/TP-001-work.md":    taskDoc("backlog", nil, nil),
	})
	idx, err := Build(set, cfg, root, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(idx.Docs))
	for i, d := range idx.Docs {
		ids[i] = d.ID
	}
	if !sort.StringsAreSorted(ids) {
		t.Errorf("docs are not sorted by ID: %v", ids)
	}
}

// ---- dogfood: build index over the real repo docs/ ----

func TestIndex_DogFood(t *testing.T) {
	// Walk up from this package to find docops.yaml.
	root := findProjectRoot(t)

	cfg, _, err := config.FindAndLoad(root)
	if err != nil {
		t.Fatalf("FindAndLoad: %v", err)
	}
	set, err := loader.Load(root, cfg)
	if err != nil {
		t.Fatalf("loader.Load: %v", err)
	}

	idx, err := Build(set, cfg, root, time.Now())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(idx.Docs) == 0 {
		t.Fatal("index is empty")
	}

	// Spot-check: ADR-0010 exists and has a non-empty implementation field.
	for _, d := range idx.Docs {
		if d.ID == "ADR-0010" {
			if d.Implementation == "" {
				t.Error("ADR-0010 should have a non-empty implementation field")
			}
			return
		}
	}
	t.Error("ADR-0010 not found in index")
}

// findProjectRoot walks up from the test file's directory to find docops.yaml.
func findProjectRoot(t *testing.T) string {
	t.Helper()
	// Start at the module root relative to this test file's package path.
	// The package is internal/index so we go up two levels.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "docops.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("docops.yaml not found walking up from", dir)
		}
		dir = parent
	}
}

// ---- helpers ----

func ctxDoc(title, typ string) string {
	return "---\ntitle: " + title + "\ntype: " + typ + "\n---\n\n" + title + " body text.\n"
}

func adrDoc(status, coverage string) string {
	return "---\ntitle: x\nstatus: " + status + "\ncoverage: " + coverage + "\ndate: 2026-04-22\n---\n\nBody text here.\n"
}

func adrDocSupersedes(status, coverage string, supersedes []string) string {
	s := "---\ntitle: x\nstatus: " + status + "\ncoverage: " + coverage + "\ndate: 2026-04-22\n"
	if len(supersedes) > 0 {
		s += "supersedes: ["
		for i, id := range supersedes {
			if i > 0 {
				s += ", "
			}
			s += id
		}
		s += "]\n"
	}
	s += "---\n\nBody text here.\n"
	return s
}

func taskDoc(status string, requires, dependsOn []string) string {
	s := "---\ntitle: x\nstatus: " + status + "\n"
	if len(requires) > 0 {
		s += "requires: ["
		for i, id := range requires {
			if i > 0 {
				s += ", "
			}
			s += id
		}
		s += "]\n"
	}
	if len(dependsOn) > 0 {
		s += "depends_on: ["
		for i, id := range dependsOn {
			if i > 0 {
				s += ", "
			}
			s += id
		}
		s += "]\n"
	}
	s += "---\n\nTask body.\n"
	return s
}
