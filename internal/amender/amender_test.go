package amender

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/logicwind/docops/internal/schema"
)

// scaffoldRoot creates a minimal docops project with the given ADR file
// at docs/decisions/<filename> and returns the project root.
func scaffoldRoot(t *testing.T, filename, body string) string {
	t.Helper()
	root := t.TempDir()
	dec := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(dec, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dec, filename), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	// docops.yaml not strictly needed for the amender (it works off Root),
	// but keep the layout realistic.
	if err := os.WriteFile(filepath.Join(root, "docops.yaml"), []byte("project: test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

const minimalADR = `---
title: Test ADR
status: accepted
coverage: required
date: "2026-04-22"
related: []
tags: []
---

## Context

Some prose.

## Decision

We will do the foo thing.

## Consequences

Stuff happens.
`

func TestRun_HappyPath_AppendsFrontmatterAndSection(t *testing.T) {
	root := scaffoldRoot(t, "ADR-0099-test.md", minimalADR)
	res, err := Run(Options{
		Root:    root,
		ADRID:   "ADR-0099",
		Kind:    "editorial",
		Summary: "fix typo in decision",
		By:      "tester",
		Date:    "2026-04-30",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.AmendmentIndex != 0 {
		t.Errorf("AmendmentIndex = %d; want 0", res.AmendmentIndex)
	}
	if !res.SectionCreated {
		t.Errorf("SectionCreated should be true on first amendment")
	}

	out, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "amendments:") {
		t.Errorf("expected amendments key in frontmatter:\n%s", got)
	}
	if !strings.Contains(got, "kind: editorial") {
		t.Errorf("expected kind: editorial:\n%s", got)
	}
	if !strings.Contains(got, "## Amendments") {
		t.Errorf("expected ## Amendments section:\n%s", got)
	}
	if !strings.Contains(got, "### 2026-04-30 — fix typo in decision (editorial)") {
		t.Errorf("expected subsection header:\n%s", got)
	}

	// Round-trip: validator should accept the result.
	fm, body, err := schema.SplitFrontmatter(out)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	a, err := schema.ParseADR(fm)
	if err != nil {
		t.Fatalf("ParseADR: %v", err)
	}
	if err := schema.ValidateADR(a); err != nil {
		t.Fatalf("ValidateADR: %v", err)
	}
	// Amendment without inline marker AND without affects_sections should
	// fail correlation (no anchor) — confirm the test fixture sees that.
	if errs := schema.ValidateAmendmentMarkers(a.Amendments, body); len(errs) == 0 {
		t.Errorf("expected unanchored-entry warning when no marker and no affects_sections")
	}
}

func TestRun_MarkerAtInsertsInline(t *testing.T) {
	root := scaffoldRoot(t, "ADR-0100-marker.md", minimalADR)
	res, err := Run(Options{
		Root:     root,
		ADRID:    "ADR-0100",
		Kind:     "editorial",
		Summary:  "rename foo to bar",
		By:       "tester",
		Date:     "2026-04-30",
		MarkerAt: "the foo thing",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.MarkerInserted {
		t.Errorf("MarkerInserted should be true")
	}

	out, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "the foo thing [AMENDED 2026-04-30 editorial]") {
		t.Errorf("expected inline marker after substring:\n%s", got)
	}

	// Validator should be happy — marker correlates with frontmatter entry.
	fm, body, _ := schema.SplitFrontmatter(out)
	a, _ := schema.ParseADR(fm)
	if errs := schema.ValidateAmendmentMarkers(a.Amendments, body); len(errs) != 0 {
		t.Errorf("expected clean markers, got %v", errs)
	}
}

func TestRun_MarkerAtNotFound(t *testing.T) {
	root := scaffoldRoot(t, "ADR-0101-mnf.md", minimalADR)
	_, err := Run(Options{
		Root:     root,
		ADRID:    "ADR-0101",
		Kind:     "editorial",
		Summary:  "x",
		By:       "tester",
		Date:     "2026-04-30",
		MarkerAt: "this string does not exist",
	})
	if err == nil {
		t.Fatal("expected MarkerNotFoundError")
	}
	var nf *MarkerNotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("expected MarkerNotFoundError, got %T: %v", err, err)
	}
}

func TestRun_MarkerAtAmbiguous(t *testing.T) {
	body := `---
title: A
status: accepted
coverage: required
date: "2026-04-22"
---

The foo bar appears here.
And the foo bar appears here too.
`
	root := scaffoldRoot(t, "ADR-0102-amb.md", body)
	_, err := Run(Options{
		Root:     root,
		ADRID:    "ADR-0102",
		Kind:     "editorial",
		Summary:  "x",
		By:       "tester",
		Date:     "2026-04-30",
		MarkerAt: "foo bar",
	})
	if err == nil {
		t.Fatal("expected MarkerAmbiguousError")
	}
	var amb *MarkerAmbiguousError
	if !errors.As(err, &amb) {
		t.Fatalf("expected MarkerAmbiguousError, got %T: %v", err, err)
	}
	if len(amb.Lines) != 2 {
		t.Errorf("expected 2 ambiguous lines, got %v", amb.Lines)
	}
}

func TestRun_AppendToExistingAmendments(t *testing.T) {
	body := `---
title: A
status: accepted
coverage: required
date: "2026-04-22"
amendments:
  - date: 2026-04-23
    kind: editorial
    by: alice
    summary: first one
    affects_sections: [Decision]
---

## Context

prose.
`
	root := scaffoldRoot(t, "ADR-0103-existing.md", body)
	res, err := Run(Options{
		Root:    root,
		ADRID:   "ADR-0103",
		Kind:    "errata",
		Summary: "second one",
		By:      "bob",
		Date:    "2026-04-30",
		AffectsSections: []string{"Context"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.AmendmentIndex != 1 {
		t.Errorf("AmendmentIndex = %d; want 1", res.AmendmentIndex)
	}
	out, _ := os.ReadFile(res.Path)
	fm, _, _ := schema.SplitFrontmatter(out)
	a, _ := schema.ParseADR(fm)
	if len(a.Amendments) != 2 {
		t.Fatalf("expected 2 amendments, got %d", len(a.Amendments))
	}
	if a.Amendments[0].Summary != "first one" || a.Amendments[1].Summary != "second one" {
		t.Errorf("ordering wrong: %+v", a.Amendments)
	}
}

func TestRun_AffectsSectionsDedupesAndPreservesOrder(t *testing.T) {
	root := scaffoldRoot(t, "ADR-0104-dedupe.md", minimalADR)
	res, err := Run(Options{
		Root:            root,
		ADRID:           "ADR-0104",
		Kind:            "clarification",
		Summary:         "x",
		By:              "tester",
		Date:            "2026-04-30",
		AffectsSections: []string{"Decision", "", "Decision", "Context"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out, _ := os.ReadFile(res.Path)
	fm, _, _ := schema.SplitFrontmatter(out)
	a, _ := schema.ParseADR(fm)
	if len(a.Amendments) != 1 || len(a.Amendments[0].AffectsSections) != 2 {
		t.Fatalf("expected 2 deduped sections, got %+v", a.Amendments)
	}
}

func TestRun_RejectsBadKind(t *testing.T) {
	root := scaffoldRoot(t, "ADR-0105-bad.md", minimalADR)
	_, err := Run(Options{Root: root, ADRID: "ADR-0105", Kind: "rewrite", Summary: "x", By: "t"})
	if err == nil || !strings.Contains(err.Error(), "kind") {
		t.Fatalf("expected kind error, got %v", err)
	}
}

func TestRun_AtomicWrite_NoPartial(t *testing.T) {
	// Smoke check: after a successful run, no .docops-amend-* tmp files
	// remain in the decisions dir.
	root := scaffoldRoot(t, "ADR-0106-atomic.md", minimalADR)
	if _, err := Run(Options{Root: root, ADRID: "ADR-0106", Kind: "editorial", Summary: "x", By: "t", Date: "2026-04-30"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(root, "docs", "decisions"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".docops-amend-") {
			t.Errorf("tmp file leaked: %s", e.Name())
		}
	}
}

func TestRun_PreservesUnrelatedFrontmatter(t *testing.T) {
	body := `---
title: Important Title
status: accepted
coverage: required
date: "2026-04-22"
related: [ADR-0001, ADR-0002]
tags: [release, scope]
---

prose.
`
	root := scaffoldRoot(t, "ADR-0107-pres.md", body)
	if _, err := Run(Options{Root: root, ADRID: "ADR-0107", Kind: "editorial", Summary: "x", By: "t", Date: "2026-04-30"}); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(filepath.Join(root, "docs", "decisions", "ADR-0107-pres.md"))
	fm, _, _ := schema.SplitFrontmatter(out)
	a, _ := schema.ParseADR(fm)
	if a.Title != "Important Title" {
		t.Errorf("title not preserved: %q", a.Title)
	}
	if len(a.Related) != 2 || a.Related[0] != "ADR-0001" {
		t.Errorf("related not preserved: %v", a.Related)
	}
	if len(a.Tags) != 2 || a.Tags[1] != "scope" {
		t.Errorf("tags not preserved: %v", a.Tags)
	}
}
