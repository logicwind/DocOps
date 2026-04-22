package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// captureStdout captures os.Stdout output from fn, restoring it afterward.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = orig

	buf := make([]byte, 1<<20)
	n, _ := r.Read(buf)
	return string(buf[:n])
}

// readTestDocs are planting helpers reused across the read-command tests.

const validCTX = `---
title: CLI substrate
type: prd
supersedes: []
---

# CLI substrate

Body of the CLI substrate context doc.
`

const validADR2 = `---
title: Use Go for implementation
status: accepted
coverage: required
date: 2026-01-01
supersedes: []
related: []
tags: [cli, go]
---

# Use Go for implementation

## Context

We need a language.

## Decision

Go.
`

const taskDone = `---
title: Scaffold the CLI
status: done
priority: p1
assignee: alice
requires: [ADR-0001]
depends_on: []
---

# Scaffold the CLI

## Goal

## Acceptance

## Notes
`

const taskBacklog1 = `---
title: Implement validate
status: backlog
priority: p1
assignee: unassigned
requires: [ADR-0001]
depends_on: [TP-001]
---

# Implement validate

## Goal

## Acceptance

## Notes
`

const taskBacklog2 = `---
title: Ship release
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0001]
depends_on: []
---

# Ship release

## Goal

## Acceptance

## Notes
`

const taskActive = `---
title: Write docs
status: active
priority: p0
assignee: bob
requires: [ADR-0001]
depends_on: []
---

# Write docs

## Goal

## Acceptance

## Notes
`

// plantReadDocs creates a full test tree and returns the root dir.
// Layout:
//
//	CTX-001  CLI substrate
//	ADR-0001 Use Go for implementation   (tags: [cli, go])
//	TP-001   Scaffold the CLI            done, p1, alice, requires ADR-0001
//	TP-002   Implement validate          backlog, p1, depends on TP-001 (blocked until TP-001 done)
//	TP-003   Ship release                backlog, p2, no deps (unblocked)
//	TP-004   Write docs                  active, p0, bob, requires ADR-0001
func plantReadDocs(t *testing.T) string {
	t.Helper()
	root := makeDocopsRoot(t)
	plantDoc(t, root, "docs/context/CTX-001-cli-substrate.md", validCTX)
	plantDoc(t, root, "docs/decisions/ADR-0001-use-go.md", validADR2)
	plantDoc(t, root, "docs/tasks/TP-001-scaffold.md", taskDone)
	plantDoc(t, root, "docs/tasks/TP-002-validate.md", taskBacklog1)
	plantDoc(t, root, "docs/tasks/TP-003-release.md", taskBacklog2)
	plantDoc(t, root, "docs/tasks/TP-004-docs.md", taskActive)
	return root
}

// ── docops get ──────────────────────────────────────────────────────────────

func TestCmdGet_HappyPath(t *testing.T) {
	plantReadDocs(t)

	out := captureStdout(t, func() {
		code := cmdGet([]string{"ADR-0001"})
		if code != 0 {
			t.Fatalf("cmdGet returned %d, want 0", code)
		}
	})

	if !strings.Contains(out, "ADR-0001") {
		t.Errorf("output missing ID: %s", out)
	}
	if !strings.Contains(out, "Use Go for implementation") {
		t.Errorf("output missing title: %s", out)
	}
	if !strings.Contains(out, "accepted") {
		t.Errorf("output missing status: %s", out)
	}
}

func TestCmdGet_NotFound(t *testing.T) {
	plantReadDocs(t)
	code := cmdGet([]string{"ADR-9999"})
	if code != 1 {
		t.Errorf("cmdGet unknown ID returned %d, want 1", code)
	}
}

func TestCmdGet_MissingArg(t *testing.T) {
	plantReadDocs(t)
	code := cmdGet(nil)
	if code != 2 {
		t.Errorf("cmdGet no args returned %d, want 2", code)
	}
}

func TestCmdGet_NoConfig(t *testing.T) {
	root := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	code := cmdGet([]string{"ADR-0001"})
	if code != 2 {
		t.Errorf("cmdGet no config returned %d, want 2", code)
	}
}

func TestCmdGet_JSON(t *testing.T) {
	plantReadDocs(t)

	out := captureStdout(t, func() {
		code := cmdGet([]string{"--json", "ADR-0001"})
		if code != 0 {
			t.Fatalf("cmdGet --json returned %d", code)
		}
	})

	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\n%s", err, out)
	}
	if doc["id"] != "ADR-0001" {
		t.Errorf("JSON id = %v, want ADR-0001", doc["id"])
	}
	if doc["kind"] != "ADR" {
		t.Errorf("JSON kind = %v, want ADR", doc["kind"])
	}
}

// ── docops list ─────────────────────────────────────────────────────────────

func TestCmdList_HappyPath(t *testing.T) {
	plantReadDocs(t)

	out := captureStdout(t, func() {
		code := cmdList(nil)
		if code != 0 {
			t.Fatalf("cmdList returned %d", code)
		}
	})

	// Should contain all six docs.
	for _, id := range []string{"CTX-001", "ADR-0001", "TP-001", "TP-002", "TP-003", "TP-004"} {
		if !strings.Contains(out, id) {
			t.Errorf("output missing %s:\n%s", id, out)
		}
	}
}

func TestCmdList_KindFilter(t *testing.T) {
	plantReadDocs(t)

	out := captureStdout(t, func() {
		cmdList([]string{"--kind", "ADR"})
	})

	if !strings.Contains(out, "ADR-0001") {
		t.Errorf("ADR-0001 missing from ADR filter: %s", out)
	}
	if strings.Contains(out, "TP-001") {
		t.Errorf("TP-001 should not appear in ADR filter: %s", out)
	}
	if strings.Contains(out, "CTX-001") {
		t.Errorf("CTX-001 should not appear in ADR filter: %s", out)
	}
}

func TestCmdList_StatusFilter(t *testing.T) {
	plantReadDocs(t)

	out := captureStdout(t, func() {
		cmdList([]string{"--status", "done"})
	})

	if !strings.Contains(out, "TP-001") {
		t.Errorf("TP-001 (done) missing: %s", out)
	}
	if strings.Contains(out, "TP-002") {
		t.Errorf("TP-002 (backlog) should not appear: %s", out)
	}
}

func TestCmdList_TagFilter(t *testing.T) {
	plantReadDocs(t)

	out := captureStdout(t, func() {
		cmdList([]string{"--tag", "go"})
	})

	if !strings.Contains(out, "ADR-0001") {
		t.Errorf("ADR-0001 (tagged go) missing: %s", out)
	}
}

func TestCmdList_TagFilterNoMatch(t *testing.T) {
	plantReadDocs(t)

	out := captureStdout(t, func() {
		cmdList([]string{"--tag", "nonexistent"})
	})

	if strings.Contains(out, "ADR-0001") {
		t.Errorf("ADR-0001 should not match tag=nonexistent: %s", out)
	}
	if !strings.Contains(out, "(no documents match)") {
		t.Errorf("expected '(no documents match)': %s", out)
	}
}

func TestCmdList_KindSortOrder(t *testing.T) {
	plantReadDocs(t)

	out := captureStdout(t, func() {
		cmdList(nil)
	})

	// CTX must appear before ADR, ADR before TP in human output.
	ctxPos := strings.Index(out, "CTX-001")
	adrPos := strings.Index(out, "ADR-0001")
	tpPos := strings.Index(out, "TP-001")
	if ctxPos < 0 || adrPos < 0 || tpPos < 0 {
		t.Fatalf("missing expected IDs in output: %s", out)
	}
	if ctxPos >= adrPos {
		t.Errorf("CTX should appear before ADR (ctxPos=%d adrPos=%d)", ctxPos, adrPos)
	}
	if adrPos >= tpPos {
		t.Errorf("ADR should appear before TP (adrPos=%d tpPos=%d)", adrPos, tpPos)
	}
}

func TestCmdList_JSON(t *testing.T) {
	plantReadDocs(t)

	out := captureStdout(t, func() {
		code := cmdList([]string{"--json"})
		if code != 0 {
			t.Fatalf("cmdList --json returned %d", code)
		}
	})

	var records []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &records); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\n%s", err, out)
	}
	if len(records) != 6 {
		t.Errorf("expected 6 records, got %d", len(records))
	}
}

func TestCmdList_JSONEmptyOnNoMatch(t *testing.T) {
	plantReadDocs(t)

	out := captureStdout(t, func() {
		cmdList([]string{"--json", "--kind", "ADR", "--status", "draft"})
	})

	var records []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &records); err != nil {
		t.Fatalf("--json output not valid JSON: %v\n%s", err, out)
	}
	if len(records) != 0 {
		t.Errorf("expected empty array, got %d records", len(records))
	}
}

func TestCmdList_CoverageFilter(t *testing.T) {
	plantReadDocs(t)

	out := captureStdout(t, func() {
		cmdList([]string{"--coverage", "required"})
	})

	if !strings.Contains(out, "ADR-0001") {
		t.Errorf("ADR-0001 (coverage=required) missing: %s", out)
	}
}

// ── docops graph ─────────────────────────────────────────────────────────────

func TestCmdGraph_HappyPath(t *testing.T) {
	plantReadDocs(t)

	out := captureStdout(t, func() {
		code := cmdGraph([]string{"ADR-0001"})
		if code != 0 {
			t.Fatalf("cmdGraph returned %d", code)
		}
	})

	// Root must be the first line.
	if !strings.HasPrefix(out, "ADR-0001") {
		t.Errorf("first line should start with ADR-0001:\n%s", out)
	}
	// Depth-1 neighbours: tasks that require ADR-0001 should appear.
	if !strings.Contains(out, "TP-001") && !strings.Contains(out, "TP-004") {
		t.Errorf("expected referencing tasks in output:\n%s", out)
	}
}

func TestCmdGraph_NotFound(t *testing.T) {
	plantReadDocs(t)
	code := cmdGraph([]string{"ADR-9999"})
	if code != 1 {
		t.Errorf("cmdGraph unknown ID returned %d, want 1", code)
	}
}

func TestCmdGraph_MissingArg(t *testing.T) {
	plantReadDocs(t)
	code := cmdGraph(nil)
	if code != 2 {
		t.Errorf("cmdGraph no args returned %d, want 2", code)
	}
}

func TestCmdGraph_DepthZero(t *testing.T) {
	plantReadDocs(t)

	out := captureStdout(t, func() {
		code := cmdGraph([]string{"--depth", "0", "ADR-0001"})
		if code != 0 {
			t.Fatalf("cmdGraph --depth 0 returned %d", code)
		}
	})

	// With depth 0 we get only the root node, no edge lines.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Errorf("depth 0 should produce 1 line, got %d:\n%s", len(lines), out)
	}
}

func TestCmdGraph_JSON(t *testing.T) {
	plantReadDocs(t)

	out := captureStdout(t, func() {
		code := cmdGraph([]string{"--json", "ADR-0001"})
		if code != 0 {
			t.Fatalf("cmdGraph --json returned %d", code)
		}
	})

	var g struct {
		Root  string                   `json:"root"`
		Nodes []map[string]interface{} `json:"nodes"`
		Edges []map[string]interface{} `json:"edges"`
	}
	if err := json.Unmarshal([]byte(out), &g); err != nil {
		t.Fatalf("--json not valid JSON: %v\n%s", err, out)
	}
	if g.Root != "ADR-0001" {
		t.Errorf("root = %q, want ADR-0001", g.Root)
	}
	if len(g.Nodes) == 0 {
		t.Error("expected at least 1 node")
	}
}

func TestCmdGraph_CycleSafe(t *testing.T) {
	// Even with mutual edges the walker must not loop.
	// TP-002 depends_on TP-001; graph from TP-001 at depth 2 should terminate.
	plantReadDocs(t)

	out := captureStdout(t, func() {
		code := cmdGraph([]string{"--depth", "2", "TP-001"})
		if code != 0 {
			t.Fatalf("cmdGraph depth 2 returned %d", code)
		}
	})

	// Just verify we got output without hanging; TP-001 must appear.
	if !strings.Contains(out, "TP-001") {
		t.Errorf("TP-001 missing from output:\n%s", out)
	}
}

// ── docops next ──────────────────────────────────────────────────────────────

func TestCmdNext_ActiveFirst(t *testing.T) {
	// TP-004 is active (bob, p0); it should win over any backlog.
	plantReadDocs(t)

	out := captureStdout(t, func() {
		code := cmdNext(nil)
		if code != 0 {
			t.Fatalf("cmdNext returned %d, want 0", code)
		}
	})

	if !strings.Contains(out, "TP-004") {
		t.Errorf("expected active TP-004, got:\n%s", out)
	}
	if !strings.Contains(out, "active for bob") {
		t.Errorf("expected 'active for bob' reason:\n%s", out)
	}
}

func TestCmdNext_UnblockedBacklog(t *testing.T) {
	// With no active tasks, TP-003 (backlog, p2, no deps) and TP-002 (backlog, p1,
	// depends_on TP-001 which is done) are both unblocked. TP-002 (p1) wins.
	root := makeDocopsRoot(t)
	plantDoc(t, root, "docs/decisions/ADR-0001-use-go.md", validADR2)
	plantDoc(t, root, "docs/tasks/TP-001-scaffold.md", taskDone)
	plantDoc(t, root, "docs/tasks/TP-002-validate.md", taskBacklog1) // p1, depends_on TP-001 (done)
	plantDoc(t, root, "docs/tasks/TP-003-release.md", taskBacklog2)  // p2, no deps

	out := captureStdout(t, func() {
		code := cmdNext(nil)
		if code != 0 {
			t.Fatalf("cmdNext returned %d, want 0", code)
		}
	})

	if !strings.Contains(out, "TP-002") {
		t.Errorf("expected TP-002 (p1 unblocked), got:\n%s", out)
	}
	if !strings.Contains(out, "unblocked backlog") {
		t.Errorf("expected 'unblocked backlog' reason:\n%s", out)
	}
}

func TestCmdNext_BlockedTaskSkipped(t *testing.T) {
	// TP-002 depends_on TP-001 which is NOT done (backlog). Only TP-003 is unblocked.
	root := makeDocopsRoot(t)
	plantDoc(t, root, "docs/decisions/ADR-0001-use-go.md", validADR2)

	const tp001Backlog = `---
title: Scaffold the CLI
status: backlog
priority: p1
assignee: unassigned
requires: [ADR-0001]
depends_on: []
---

# Scaffold the CLI

## Goal

## Acceptance

## Notes
`
	plantDoc(t, root, "docs/tasks/TP-001-scaffold.md", tp001Backlog)
	plantDoc(t, root, "docs/tasks/TP-002-validate.md", taskBacklog1)  // depends_on TP-001 (not done)
	plantDoc(t, root, "docs/tasks/TP-003-release.md", taskBacklog2)    // p2, no deps

	out := captureStdout(t, func() {
		code := cmdNext(nil)
		if code != 0 {
			t.Fatalf("cmdNext returned %d", code)
		}
	})

	// TP-002 is blocked (TP-001 not done). Should pick TP-001 (p1) or TP-003 (p2).
	// Both TP-001 and TP-003 are unblocked; TP-001 is p1 so it wins.
	if strings.Contains(out, "TP-002") {
		t.Errorf("blocked TP-002 should not be selected:\n%s", out)
	}
}

func TestCmdNext_PriorityOrder(t *testing.T) {
	// Three unblocked backlog tasks at p0, p1, p2. p0 must win.
	root := makeDocopsRoot(t)
	plantDoc(t, root, "docs/decisions/ADR-0001-use-go.md", validADR2)

	tasks := []struct {
		file    string
		content string
	}{
		{"docs/tasks/TP-001-p0.md", `---
title: P0 task
status: backlog
priority: p0
assignee: unassigned
requires: [ADR-0001]
depends_on: []
---

# P0 task

## Goal

## Acceptance

## Notes
`},
		{"docs/tasks/TP-002-p1.md", `---
title: P1 task
status: backlog
priority: p1
assignee: unassigned
requires: [ADR-0001]
depends_on: []
---

# P1 task

## Goal

## Acceptance

## Notes
`},
		{"docs/tasks/TP-003-p2.md", `---
title: P2 task
status: backlog
priority: p2
assignee: unassigned
requires: [ADR-0001]
depends_on: []
---

# P2 task

## Goal

## Acceptance

## Notes
`},
	}
	for _, tc := range tasks {
		plantDoc(t, root, tc.file, tc.content)
	}

	out := captureStdout(t, func() {
		code := cmdNext(nil)
		if code != 0 {
			t.Fatalf("cmdNext returned %d", code)
		}
	})

	if !strings.Contains(out, "TP-001") {
		t.Errorf("p0 task TP-001 should win, got:\n%s", out)
	}
}

func TestCmdNext_AssigneeFilter(t *testing.T) {
	// --assignee bob: should find TP-004 (active, bob).
	plantReadDocs(t)

	out := captureStdout(t, func() {
		code := cmdNext([]string{"--assignee", "bob"})
		if code != 0 {
			t.Fatalf("cmdNext --assignee bob returned %d", code)
		}
	})

	if !strings.Contains(out, "TP-004") {
		t.Errorf("expected TP-004 for assignee=bob:\n%s", out)
	}
}

func TestCmdNext_NoMatch(t *testing.T) {
	// Repo with only done tasks — nothing to pick.
	root := makeDocopsRoot(t)
	plantDoc(t, root, "docs/decisions/ADR-0001-use-go.md", validADR2)
	plantDoc(t, root, "docs/tasks/TP-001-scaffold.md", taskDone)

	code := cmdNext(nil)
	if code != 1 {
		t.Errorf("cmdNext with only done tasks returned %d, want 1", code)
	}
}

func TestCmdNext_JSON(t *testing.T) {
	// TP-004 is active — should be selected.
	plantReadDocs(t)

	out := captureStdout(t, func() {
		code := cmdNext([]string{"--json"})
		if code != 0 {
			t.Fatalf("cmdNext --json returned %d", code)
		}
	})

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("--json not valid JSON: %v\n%s", err, out)
	}
	if result["id"] != "TP-004" {
		t.Errorf("JSON id = %v, want TP-004", result["id"])
	}
	if result["reason"] == "" || result["reason"] == nil {
		t.Errorf("JSON missing reason field: %v", result)
	}
}
