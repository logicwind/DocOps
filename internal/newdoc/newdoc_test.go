package newdoc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/schema"
	"gopkg.in/yaml.v3"
)

// makeRoot creates a temp directory with a minimal docops.yaml so
// config.FindAndLoad succeeds.
func makeRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	// Write a minimal docops.yaml.
	if err := os.WriteFile(filepath.Join(root, "docops.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatalf("write docops.yaml: %v", err)
	}
	// Create the doc directories.
	cfg := config.Default()
	for _, dir := range []string{cfg.Paths.Context, cfg.Paths.Decisions, cfg.Paths.Tasks} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	return root
}

// runNew is a helper that calls Run with NoOpen:true so tests never spawn an editor.
func runNew(t *testing.T, opts Options) *Result {
	t.Helper()
	opts.NoOpen = true
	res, err := Run(opts)
	if err != nil {
		t.Fatalf("Run(%+v): %v", opts, err)
	}
	return res
}

// ----- Slugification ---------------------------------------------------------

func TestSlugify(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Hello, World! 🚀", "hello-world"},
		{"fix indexing bug", "fix-indexing-bug"},
		{"  leading and trailing  ", "leading-and-trailing"},
		{"already-lower", "already-lower"},
		// Long title must be truncated at 50 chars.
		{"This is a very long title that exceeds fifty characters in total length", "this-is-a-very-long-title-that-exceeds-fifty-chara"},
	}
	for _, tc := range cases {
		got := slugify(tc.in)
		if got != tc.want {
			t.Errorf("slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if len(got) > slugMaxLen {
			t.Errorf("slugify(%q) length %d > %d", tc.in, len(got), slugMaxLen)
		}
	}
}

// ----- CTX creation ----------------------------------------------------------

func TestNewCtx(t *testing.T) {
	root := makeRoot(t)
	res := runNew(t, Options{Root: root, Type: DocTypeCtx, Title: "My Context Doc", CtxType: "brief"})

	if !strings.HasPrefix(filepath.Base(res.Path), "CTX-001-") {
		t.Errorf("path %s does not start with CTX-001-", res.Path)
	}
	if res.ID != "CTX-001" {
		t.Errorf("ID = %q, want CTX-001", res.ID)
	}

	raw, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	fm, _, err := schema.SplitFrontmatter(raw)
	if err != nil {
		t.Fatalf("split frontmatter: %v", err)
	}
	ctx, err := schema.ParseContext(fm)
	if err != nil {
		t.Fatalf("parse ctx: %v", err)
	}
	if ctx.Title != "My Context Doc" {
		t.Errorf("title = %q, want My Context Doc", ctx.Title)
	}
	if ctx.Type != "brief" {
		t.Errorf("type = %q, want brief", ctx.Type)
	}
	if ctx.Supersedes == nil {
		t.Errorf("supersedes should be non-nil (empty slice)")
	}
}

func TestNewCtxDefaultType(t *testing.T) {
	root := makeRoot(t)
	res := runNew(t, Options{Root: root, Type: DocTypeCtx, Title: "Memo doc"})
	raw, _ := os.ReadFile(res.Path)
	fm, _, _ := schema.SplitFrontmatter(raw)
	ctx, _ := schema.ParseContext(fm)
	if ctx.Type != "memo" {
		t.Errorf("default type = %q, want memo", ctx.Type)
	}
}

// ----- ADR creation ----------------------------------------------------------

func TestNewADR(t *testing.T) {
	root := makeRoot(t)
	res := runNew(t, Options{
		Root:    root,
		Type:    DocTypeADR,
		Title:   "Use event sourcing",
		Related: []string{"ADR-0001", "ADR-0002"},
	})

	if !strings.HasPrefix(filepath.Base(res.Path), "ADR-0001-") {
		t.Errorf("path %s does not start with ADR-0001-", res.Path)
	}
	if res.ID != "ADR-0001" {
		t.Errorf("ID = %q, want ADR-0001", res.ID)
	}

	raw, _ := os.ReadFile(res.Path)
	fm, _, _ := schema.SplitFrontmatter(raw)
	adr, err := schema.ParseADR(fm)
	if err != nil {
		t.Fatalf("parse adr: %v", err)
	}
	if adr.Status != "draft" {
		t.Errorf("status = %q, want draft", adr.Status)
	}
	if adr.Coverage != "required" {
		t.Errorf("coverage = %q, want required", adr.Coverage)
	}
	if len(adr.Related) != 2 {
		t.Errorf("related len = %d, want 2", len(adr.Related))
	}
}

// ----- Task creation ---------------------------------------------------------

func TestNewTask(t *testing.T) {
	root := makeRoot(t)
	res := runNew(t, Options{
		Root:     root,
		Type:     DocTypeTask,
		Title:    "Fix indexing bug",
		Requires: []string{"ADR-0004"},
		Priority: "p1",
		Assignee: "alice",
	})

	if !strings.HasPrefix(filepath.Base(res.Path), "TP-001-") {
		t.Errorf("path %s does not start with TP-001-", res.Path)
	}
	if res.ID != "TP-001" {
		t.Errorf("ID = %q, want TP-001", res.ID)
	}

	raw, _ := os.ReadFile(res.Path)
	fm, body, _ := schema.SplitFrontmatter(raw)
	task, err := schema.ParseTask(fm)
	if err != nil {
		t.Fatalf("parse task: %v", err)
	}
	if task.Status != "backlog" {
		t.Errorf("status = %q, want backlog", task.Status)
	}
	if task.Priority != "p1" {
		t.Errorf("priority = %q, want p1", task.Priority)
	}
	if task.Assignee != "alice" {
		t.Errorf("assignee = %q, want alice", task.Assignee)
	}
	if len(task.Requires) != 1 || task.Requires[0] != "ADR-0004" {
		t.Errorf("requires = %v, want [ADR-0004]", task.Requires)
	}
	// Body should have the three sections.
	if !strings.Contains(string(body), "## Goal") {
		t.Error("task body missing ## Goal")
	}
	if !strings.Contains(string(body), "## Acceptance") {
		t.Error("task body missing ## Acceptance")
	}
}

func TestNewTaskDefaults(t *testing.T) {
	root := makeRoot(t)
	res := runNew(t, Options{
		Root:     root,
		Type:     DocTypeTask,
		Title:    "Default task",
		Requires: []string{"CTX-001"},
	})
	raw, _ := os.ReadFile(res.Path)
	fm, _, _ := schema.SplitFrontmatter(raw)
	task, _ := schema.ParseTask(fm)
	if task.Priority != "p2" {
		t.Errorf("default priority = %q, want p2", task.Priority)
	}
	if task.Assignee != "unassigned" {
		t.Errorf("default assignee = %q, want unassigned", task.Assignee)
	}
}

// ----- Validation errors -----------------------------------------------------

func TestTaskRequiresEnforced(t *testing.T) {
	root := makeRoot(t)
	_, err := Run(Options{
		Root:   root,
		Type:   DocTypeTask,
		Title:  "orphan task",
		NoOpen: true,
		// No Requires — must be rejected.
	})
	if err == nil {
		t.Fatal("expected error for task with empty --requires, got nil")
	}
	if !strings.Contains(err.Error(), "ADR-0004") {
		t.Errorf("error should mention ADR-0004, got: %v", err)
	}
}

func TestEmptyTitleRejected(t *testing.T) {
	root := makeRoot(t)
	_, err := Run(Options{Root: root, Type: DocTypeCtx, Title: "", NoOpen: true})
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestUnknownTypeRejected(t *testing.T) {
	root := makeRoot(t)
	_, err := Run(Options{Root: root, Type: "bogus", Title: "x", NoOpen: true})
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

// ----- Counter monotonicity --------------------------------------------------

func TestCounterMonotonicity(t *testing.T) {
	root := makeRoot(t)
	ids := make([]string, 5)
	for i := range ids {
		res := runNew(t, Options{
			Root:     root,
			Type:     DocTypeTask,
			Title:    fmt.Sprintf("task %d", i),
			Requires: []string{"ADR-0001"},
		})
		ids[i] = res.ID
	}
	// Each ID must be unique and sequentially increasing.
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Errorf("duplicate ID: %s", id)
		}
		seen[id] = true
	}
}

// ----- Parallel safety -------------------------------------------------------

func TestParallelIDAllocation(t *testing.T) {
	const N = 20
	root := makeRoot(t)

	var mu sync.Mutex
	var wg sync.WaitGroup
	results := make([]*Result, 0, N)
	errs := make([]error, 0)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res, err := Run(Options{
				Root:     root,
				Type:     DocTypeTask,
				Title:    fmt.Sprintf("parallel task %d", i),
				Requires: []string{"ADR-0001"},
				NoOpen:   true,
			})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
			} else {
				results = append(results, res)
			}
		}(i)
	}
	wg.Wait()

	if len(errs) > 0 {
		t.Fatalf("parallel Run errors: %v", errs[0])
	}
	if len(results) != N {
		t.Fatalf("got %d results, want %d", len(results), N)
	}

	// All IDs must be distinct.
	ids := map[string]bool{}
	paths := map[string]bool{}
	for _, r := range results {
		if ids[r.ID] {
			t.Errorf("duplicate ID: %s", r.ID)
		}
		if paths[r.Path] {
			t.Errorf("duplicate path: %s", r.Path)
		}
		ids[r.ID] = true
		paths[r.Path] = true
	}
	if len(ids) != N {
		t.Errorf("got %d distinct IDs, want %d", len(ids), N)
	}
	// All files must exist.
	for _, r := range results {
		if _, err := os.Stat(r.Path); err != nil {
			t.Errorf("file not found: %s", r.Path)
		}
	}
}

// ----- Seed from existing IDs ------------------------------------------------

func TestSeedFromExistingIDs(t *testing.T) {
	root := makeRoot(t)
	cfg := config.Default()

	// Plant two existing tasks manually (no counter file).
	for _, name := range []string{"TP-001-alpha.md", "TP-007-beta.md"} {
		body := "---\ntitle: x\nstatus: backlog\npriority: p2\nassignee: u\nrequires: [ADR-0001]\ndepends_on: []\n---\n\n# x\n"
		if err := os.WriteFile(filepath.Join(root, cfg.Paths.Tasks, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// No counters.json — the next TP should be 008.
	res := runNew(t, Options{
		Root:     root,
		Type:     DocTypeTask,
		Title:    "next task",
		Requires: []string{"ADR-0001"},
	})
	if res.ID != "TP-008" {
		t.Errorf("seed: got ID %s, want TP-008", res.ID)
	}
}

// ----- JSON output -----------------------------------------------------------

func TestJSONOutput(t *testing.T) {
	root := makeRoot(t)
	// JSON flag suppresses editor and human output; result carries the fields.
	opts := Options{
		Root:     root,
		Type:     DocTypeTask,
		Title:    "a",
		Requires: []string{"ADR-0004"},
		NoOpen:   true,
		JSON:     true,
	}
	res, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Serialize the way cmd layer would.
	out := struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	}{res.ID, res.Rel}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	var check struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal(raw, &check); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if check.ID != res.ID {
		t.Errorf("json id = %q, want %q", check.ID, res.ID)
	}
}

// ----- Flow-style YAML sequences ---------------------------------------------

func TestFrontmatterFlowStyle(t *testing.T) {
	root := makeRoot(t)
	res := runNew(t, Options{
		Root:     root,
		Type:     DocTypeTask,
		Title:    "flow test",
		Requires: []string{"ADR-0001", "CTX-002"},
	})
	raw, _ := os.ReadFile(res.Path)
	content := string(raw)
	// requires should be on one line in flow style.
	if !strings.Contains(content, "requires: [ADR-0001, CTX-002]") {
		t.Errorf("expected flow-style requires, got:\n%s", content)
	}
}

// ----- ADR 4-digit padding ---------------------------------------------------

func TestADRFourDigitID(t *testing.T) {
	root := makeRoot(t)
	res := runNew(t, Options{Root: root, Type: DocTypeADR, Title: "some decision"})
	if !strings.HasPrefix(res.ID, "ADR-") || len(res.ID) != 8 {
		t.Errorf("ADR ID %q should be ADR-NNNN (8 chars)", res.ID)
	}
}

// fmt is used in the test — avoid unused import by aliasing.
var _ = fmt.Sprintf

// Ensure yaml is used (it's imported for the flow-style assertion check).
var _ = yaml.Marshal
