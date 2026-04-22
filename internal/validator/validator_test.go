package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/loader"
)

// scenario builds a throwaway project with the given files relative to
// the temp root and returns a loaded DocSet plus the config.
func scenario(t *testing.T, files map[string]string) (*loader.DocSet, config.Config) {
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
		t.Fatalf("Load: %v", err)
	}
	return set, cfg
}

func TestValidate_HappyPath(t *testing.T) {
	set, cfg := scenario(t, map[string]string{
		"docs/context/CTX-001-v.md":      ctx("vision", "brief"),
		"docs/decisions/ADR-0001-x.md":   adr("accepted"),
		"docs/tasks/TP-001-work.md":      task("backlog", []string{"ADR-0001"}, nil),
	})
	r := Validate(set, cfg)
	if !r.OK() {
		t.Fatalf("expected OK, got errors: %+v", r.Errors)
	}
}

func TestValidate_UnresolvedRef(t *testing.T) {
	set, cfg := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-x.md": adr("accepted"),
		"docs/tasks/TP-001-work.md":    task("backlog", []string{"ADR-0999"}, nil),
	})
	r := Validate(set, cfg)
	if r.OK() {
		t.Fatalf("expected errors")
	}
	if !containsRule(r.Errors, "reference-unresolved") {
		t.Fatalf("missing reference-unresolved rule: %+v", r.Errors)
	}
}

func TestValidate_CycleInDependsOn(t *testing.T) {
	set, cfg := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-x.md": adr("accepted"),
		"docs/tasks/TP-001-a.md":       task("backlog", []string{"ADR-0001"}, []string{"TP-002"}),
		"docs/tasks/TP-002-b.md":       task("backlog", []string{"ADR-0001"}, []string{"TP-001"}),
	})
	r := Validate(set, cfg)
	if !containsRule(r.Errors, "cycle") {
		t.Fatalf("expected cycle error, got: %+v", r.Errors)
	}
}

func TestValidate_SupersededCitation_WarnsByDefault(t *testing.T) {
	set, cfg := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-old.md": adr("superseded"),
		"docs/tasks/TP-001-work.md":      task("backlog", []string{"ADR-0001"}, nil),
	})
	r := Validate(set, cfg)
	if !r.OK() {
		t.Fatalf("expected no errors, got: %+v", r.Errors)
	}
	if !containsRule(r.Warnings, "citation-superseded") {
		t.Fatalf("expected citation-superseded warning: %+v", r.Warnings)
	}
}

func TestValidate_SupersededCitation_ErrorWhenConfigured(t *testing.T) {
	set, cfg := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-old.md": adr("superseded"),
		"docs/tasks/TP-001-work.md":      task("backlog", []string{"ADR-0001"}, nil),
	})
	cfg.Gaps.TaskRequiresSupersededAdr = config.SeverityError
	r := Validate(set, cfg)
	if !containsRule(r.Errors, "citation-superseded") {
		t.Fatalf("expected citation-superseded error: %+v", r.Errors)
	}
}

func TestValidate_SchemaErrorSurfaces(t *testing.T) {
	set, cfg := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-x.md": adr("in-review"), // invalid enum
	})
	r := Validate(set, cfg)
	if r.OK() {
		t.Fatalf("expected schema error")
	}
	if !containsRule(r.Errors, "schema") {
		t.Fatalf("expected schema rule: %+v", r.Errors)
	}
}

func TestValidate_DeterministicOrder(t *testing.T) {
	// Two unrelated errors; output must be stable across runs.
	set, cfg := scenario(t, map[string]string{
		"docs/decisions/ADR-0001-x.md": adr("in-review"),
		"docs/decisions/ADR-0002-y.md": adr("in-review"),
	})
	r1 := Validate(set, cfg)
	r2 := Validate(set, cfg)
	if len(r1.Errors) != len(r2.Errors) {
		t.Fatalf("length differs")
	}
	for i := range r1.Errors {
		if r1.Errors[i] != r2.Errors[i] {
			t.Errorf("index %d differs: %v vs %v", i, r1.Errors[i], r2.Errors[i])
		}
	}
}

// ---- helpers ----

func containsRule(fs []Finding, rule string) bool {
	for _, f := range fs {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

func ctx(title, typ string) string {
	return "---\ntitle: " + title + "\ntype: " + typ + "\n---\nbody\n"
}

func adr(status string) string {
	return "---\ntitle: x\nstatus: " + status + "\ncoverage: required\ndate: 2026-04-22\n---\nbody\n"
}

func task(status string, requires, dependsOn []string) string {
	var sb strings.Builder
	sb.WriteString("---\ntitle: x\nstatus: " + status + "\n")
	sb.WriteString("requires: [" + strings.Join(requires, ", ") + "]\n")
	if len(dependsOn) > 0 {
		sb.WriteString("depends_on: [" + strings.Join(dependsOn, ", ") + "]\n")
	}
	sb.WriteString("---\nbody\n")
	return sb.String()
}
