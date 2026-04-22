package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// plantSearchDocs builds a tree with content suitable for search tests.
// CTX-001 (prd) — "CLI substrate"
// ADR-0001 — "Use Go for implementation"  tags:[cli, go]
// ADR-0002 — "Rate limiting strategy"     tags:[api, rate-limit]
// TP-001   — done, alice, "Scaffold the CLI"   requires ADR-0001
// TP-002   — backlog, "Implement rate limiter" requires ADR-0002
func plantSearchDocs(t *testing.T) string {
	t.Helper()
	root := makeDocopsRoot(t)
	plantDoc(t, root, "docs/context/CTX-001-cli-substrate.md", validCTX)
	plantDoc(t, root, "docs/decisions/ADR-0001-use-go.md", validADR2)
	plantDoc(t, root, "docs/decisions/ADR-0002-rate-limit.md", `---
title: Rate limiting strategy
status: draft
coverage: required
date: 2026-01-02
supersedes: []
related: []
tags: [api, rate-limit]
---

# Rate limiting strategy

## Context

We need to protect our API endpoints from abuse.

## Decision

Token bucket algorithm for rate limiting.
`)
	plantDoc(t, root, "docs/tasks/TP-001-scaffold.md", taskDone)
	plantDoc(t, root, "docs/tasks/TP-002-rate-limiter.md", `---
title: Implement rate limiter
status: backlog
priority: p1
assignee: unassigned
requires: [ADR-0002]
depends_on: []
---

# Implement rate limiter

## Goal

Ship the token bucket rate limiter.

## Acceptance

## Notes
`)
	return root
}

// ── basic text match ─────────────────────────────────────────────────────────

func TestCmdSearch_TitleMatch(t *testing.T) {
	plantSearchDocs(t)
	out := captureStdout(t, func() {
		code := cmdSearch([]string{"Go"})
		if code != 0 {
			t.Fatalf("cmdSearch returned %d", code)
		}
	})
	if !strings.Contains(out, "ADR-0001") {
		t.Errorf("ADR-0001 (title 'Use Go') missing:\n%s", out)
	}
	if !strings.Contains(out, "title") {
		t.Errorf("match_field 'title' missing from output:\n%s", out)
	}
}

func TestCmdSearch_TagMatch(t *testing.T) {
	plantSearchDocs(t)
	out := captureStdout(t, func() {
		cmdSearch([]string{"rate-limit"})
	})
	if !strings.Contains(out, "ADR-0002") {
		t.Errorf("ADR-0002 (tag 'rate-limit') missing:\n%s", out)
	}
	if !strings.Contains(out, "tags") {
		t.Errorf("match_field 'tags' missing:\n%s", out)
	}
}

func TestCmdSearch_BodyMatch(t *testing.T) {
	plantSearchDocs(t)
	out := captureStdout(t, func() {
		cmdSearch([]string{"token bucket"})
	})
	if !strings.Contains(out, "ADR-0002") {
		t.Errorf("ADR-0002 (body 'token bucket') missing:\n%s", out)
	}
	if !strings.Contains(out, "body") {
		t.Errorf("match_field 'body' missing:\n%s", out)
	}
}

func TestCmdSearch_CaseInsensitiveDefault(t *testing.T) {
	plantSearchDocs(t)
	out := captureStdout(t, func() {
		cmdSearch([]string{"go for implementation"})
	})
	if !strings.Contains(out, "ADR-0001") {
		t.Errorf("case-insensitive match failed:\n%s", out)
	}
}

func TestCmdSearch_CaseSensitive(t *testing.T) {
	plantSearchDocs(t)
	out := captureStdout(t, func() {
		cmdSearch([]string{"--case", "USE GO"}) // uppercase won't match lowercase title
	})
	if strings.Contains(out, "ADR-0001") {
		t.Errorf("case-sensitive search should not match 'Use Go' with 'USE GO':\n%s", out)
	}
}

func TestCmdSearch_RegexMode(t *testing.T) {
	plantSearchDocs(t)
	out := captureStdout(t, func() {
		cmdSearch([]string{"--regex", "rat(e|ing)"})
	})
	if !strings.Contains(out, "ADR-0002") {
		t.Errorf("regex match failed:\n%s", out)
	}
}

func TestCmdSearch_InvalidRegex(t *testing.T) {
	plantSearchDocs(t)
	code := cmdSearch([]string{"--regex", "["})
	if code != 2 {
		t.Errorf("invalid regex should return 2, got %d", code)
	}
}

// ── structured filters ───────────────────────────────────────────────────────

func TestCmdSearch_KindFilter(t *testing.T) {
	plantSearchDocs(t)
	out := captureStdout(t, func() {
		cmdSearch([]string{"--kind", "ADR", "rate"})
	})
	if strings.Contains(out, "TP-") {
		t.Errorf("TP docs should be excluded by --kind ADR:\n%s", out)
	}
	if !strings.Contains(out, "ADR-0002") {
		t.Errorf("ADR-0002 should match:\n%s", out)
	}
}

func TestCmdSearch_StatusFilter(t *testing.T) {
	plantSearchDocs(t)
	out := captureStdout(t, func() {
		cmdSearch([]string{"--kind", "ADR", "--status", "draft", "rate"})
	})
	if !strings.Contains(out, "ADR-0002") {
		t.Errorf("ADR-0002 (draft) missing:\n%s", out)
	}
	if strings.Contains(out, "ADR-0001") {
		t.Errorf("ADR-0001 (accepted) should be excluded:\n%s", out)
	}
}

func TestCmdSearch_CoverageFilter(t *testing.T) {
	plantSearchDocs(t)
	out := captureStdout(t, func() {
		cmdSearch([]string{"--coverage", "required", "rate"})
	})
	if !strings.Contains(out, "ADR-0002") {
		t.Errorf("ADR-0002 missing with coverage=required:\n%s", out)
	}
}

func TestCmdSearch_TagStructuredFilter(t *testing.T) {
	plantSearchDocs(t)
	out := captureStdout(t, func() {
		cmdSearch([]string{"--tag", "api", "rate"})
	})
	if !strings.Contains(out, "ADR-0002") {
		t.Errorf("ADR-0002 missing with --tag api:\n%s", out)
	}
	if strings.Contains(out, "ADR-0001") {
		t.Errorf("ADR-0001 should be excluded (no api tag):\n%s", out)
	}
}

func TestCmdSearch_FilterOnly(t *testing.T) {
	plantSearchDocs(t)
	out := captureStdout(t, func() {
		code := cmdSearch([]string{"--kind", "ADR"})
		if code != 0 {
			t.Fatalf("filter-only returned %d", code)
		}
	})
	if !strings.Contains(out, "ADR-0001") || !strings.Contains(out, "ADR-0002") {
		t.Errorf("both ADRs should appear in filter-only:\n%s", out)
	}
}

func TestCmdSearch_NoQueryNoFilter(t *testing.T) {
	plantSearchDocs(t)
	code := cmdSearch(nil)
	if code != 2 {
		t.Errorf("no query no filter should return 2, got %d", code)
	}
}

func TestCmdSearch_CTXStatusError(t *testing.T) {
	plantSearchDocs(t)
	code := cmdSearch([]string{"--kind", "CTX", "--status", "accepted", "cli"})
	if code != 2 {
		t.Errorf("CTX+status should return 2, got %d", code)
	}
}

// ── ranking ──────────────────────────────────────────────────────────────────

func TestCmdSearch_RankingTitleBeforeBody(t *testing.T) {
	// "Rate limiting" appears in ADR-0002 title AND body.
	// Title match should rank first.
	plantSearchDocs(t)
	out := captureStdout(t, func() {
		cmdSearch([]string{"rate"})
	})
	// ADR-0002 should appear; its match should be "title" not "body"
	if !strings.Contains(out, "ADR-0002") {
		t.Fatalf("ADR-0002 missing:\n%s", out)
	}
	// In the output, ADR-0002's line should show (title) snippet
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		if strings.Contains(l, "ADR-0002") && i+1 < len(lines) {
			if strings.Contains(lines[i+1], "(title)") {
				return // pass
			}
		}
	}
	t.Errorf("expected (title) match for ADR-0002:\n%s", out)
}

// ── output shapes ────────────────────────────────────────────────────────────

func TestCmdSearch_JSON(t *testing.T) {
	plantSearchDocs(t)
	out := captureStdout(t, func() {
		code := cmdSearch([]string{"--json", "rate"})
		if code != 0 {
			t.Fatalf("--json returned %d", code)
		}
	})
	var results []searchResult
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("--json not valid JSON: %v\n%s", err, out)
	}
	if len(results) == 0 {
		t.Error("expected at least one result")
	}
	for _, r := range results {
		if r.ID == "" || r.Kind == "" || r.MatchField == "" {
			t.Errorf("result missing required fields: %+v", r)
		}
	}
}

func TestCmdSearch_JSONEmptyOnNoMatch(t *testing.T) {
	plantSearchDocs(t)
	out := captureStdout(t, func() {
		cmdSearch([]string{"--json", "xyzzy-no-match-anywhere"})
	})
	var results []searchResult
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("--json not valid JSON: %v\n%s", err, out)
	}
	if len(results) != 0 {
		t.Errorf("expected empty array, got %d results", len(results))
	}
}

func TestCmdSearch_SummaryLine(t *testing.T) {
	plantSearchDocs(t)
	out := captureStdout(t, func() {
		cmdSearch([]string{"rate"})
	})
	if !strings.Contains(out, "match(es)") {
		t.Errorf("missing trailing 'N match(es)' line:\n%s", out)
	}
}

func TestCmdSearch_NoConfig(t *testing.T) {
	root := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	code := cmdSearch([]string{"anything"})
	if code != 2 {
		t.Errorf("no config should return 2, got %d", code)
	}
}

// ── dog-food ─────────────────────────────────────────────────────────────────

func TestCmdSearch_Dogfood(t *testing.T) {
	// Run against the real project repo. Only works when cwd is the repo root.
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })

	out := captureStdout(t, func() {
		code := cmdSearch([]string{"ADR-0018"})
		if code != 0 {
			t.Fatalf("dog-food search returned %d", code)
		}
	})
	if !strings.Contains(out, "match(es)") {
		t.Errorf("dog-food: missing summary line:\n%s", out)
	}
}
