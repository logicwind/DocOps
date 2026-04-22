package scaffold

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractBlock_HappyPath(t *testing.T) {
	tmpl := []byte("preamble\n" + BlockStart + "\nbody\n" + BlockEnd + "\ntrailer\n")
	got := ExtractBlock(tmpl)
	want := "\nbody\n"
	if got != want {
		t.Errorf("ExtractBlock = %q; want %q", got, want)
	}
}

func TestExtractBlock_MissingMarkers(t *testing.T) {
	cases := map[string][]byte{
		"no markers":      []byte("just plain text\n"),
		"only start":      []byte("hello\n" + BlockStart + "\nrest\n"),
		"only end":        []byte("hello\n" + BlockEnd + "\nrest\n"),
		"end before start": []byte(BlockEnd + "\n" + BlockStart + "\n"),
	}
	for name, tmpl := range cases {
		t.Run(name, func(t *testing.T) {
			if got := ExtractBlock(tmpl); got != "" {
				t.Errorf("ExtractBlock(%q) = %q; want empty", name, got)
			}
		})
	}
}

func TestMergeAgentsBlock_AppendsWhenAbsent(t *testing.T) {
	existing := []byte("# Project\n\nHand-written notes.\n")
	tmpl := []byte(BlockStart + "\nshipped block body\n" + BlockEnd + "\n")

	merged, changed, reason := MergeAgentsBlock(existing, tmpl)
	if !changed {
		t.Fatalf("expected changed=true; reason=%q", reason)
	}
	s := string(merged)
	if !strings.Contains(s, "Hand-written notes.") {
		t.Errorf("user content lost: %s", s)
	}
	if !strings.Contains(s, BlockStart) || !strings.Contains(s, BlockEnd) {
		t.Errorf("block delimiters missing: %s", s)
	}
	if !strings.Contains(s, "shipped block body") {
		t.Errorf("template body missing: %s", s)
	}
}

func TestMergeAgentsBlock_ReplacesInPlace(t *testing.T) {
	existing := []byte("# Header\n\n" + BlockStart + "\nstale\n" + BlockEnd + "\n\nuser footer\n")
	tmpl := []byte(BlockStart + "\nfresh\n" + BlockEnd + "\n")

	merged, changed, reason := MergeAgentsBlock(existing, tmpl)
	if !changed {
		t.Fatalf("expected changed=true; reason=%q", reason)
	}
	s := string(merged)
	if strings.Contains(s, "stale") {
		t.Errorf("stale block content survived: %s", s)
	}
	if !strings.Contains(s, "fresh") {
		t.Errorf("fresh block content missing: %s", s)
	}
	if !strings.Contains(s, "user footer") {
		t.Errorf("user content outside block was lost: %s", s)
	}
}

func TestMergeAgentsBlock_NoOpWhenIdentical(t *testing.T) {
	body := BlockStart + "\nshared body\n" + BlockEnd + "\n"
	existing := []byte("# Header\n\n" + body + "\nfooter\n")
	tmpl := []byte(body)

	merged, changed, reason := MergeAgentsBlock(existing, tmpl)
	if changed {
		t.Fatalf("expected changed=false when block matches; reason=%q", reason)
	}
	if !bytes.Equal(merged, existing) {
		t.Errorf("merged should equal existing on no-op")
	}
}

func TestMergeAgentsBlock_RefusesMalformedTemplate(t *testing.T) {
	existing := []byte("# Header\n")
	tmpl := []byte("template with no markers at all\n")

	_, changed, reason := MergeAgentsBlock(existing, tmpl)
	if changed {
		t.Errorf("expected changed=false on malformed template")
	}
	if !strings.Contains(reason, "template missing") {
		t.Errorf("expected reason about missing template block, got %q", reason)
	}
}

func TestFileAction_CreateWhenAbsent(t *testing.T) {
	root := t.TempDir()
	a := FileAction(root, "new.txt", []byte("hello"), 0o644, false)
	if a.Kind != KindWriteFile {
		t.Errorf("Kind = %q; want %q", a.Kind, KindWriteFile)
	}
	if a.Reason != "create" {
		t.Errorf("Reason = %q; want %q", a.Reason, "create")
	}
}

func TestFileAction_SkipWhenIdentical(t *testing.T) {
	root := t.TempDir()
	body := []byte("same body\n")
	if err := os.WriteFile(filepath.Join(root, "same.txt"), body, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	a := FileAction(root, "same.txt", body, 0o644, false)
	if a.Kind != KindSkip {
		t.Errorf("Kind = %q; want %q", a.Kind, KindSkip)
	}
	if !strings.Contains(a.Reason, "up to date") {
		t.Errorf("Reason = %q; want 'up to date' phrasing", a.Reason)
	}
}

func TestFileAction_SkipWhenDriftedNoForce(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "drift.txt"), []byte("old"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	a := FileAction(root, "drift.txt", []byte("new"), 0o644, false)
	if a.Kind != KindSkip {
		t.Errorf("Kind = %q; want %q (skip on drift without --force)", a.Kind, KindSkip)
	}
	if !strings.Contains(a.Reason, "--force") {
		t.Errorf("Reason should hint at --force, got %q", a.Reason)
	}
}

func TestFileAction_OverwriteWhenDriftedWithForce(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "drift.txt"), []byte("old"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	a := FileAction(root, "drift.txt", []byte("new"), 0o644, true)
	if a.Kind != KindWriteFile {
		t.Errorf("Kind = %q; want %q (overwrite with --force)", a.Kind, KindWriteFile)
	}
	if !strings.Contains(a.Reason, "overwrite") {
		t.Errorf("Reason should describe overwrite, got %q", a.Reason)
	}
}

func TestDirAction_CreateWhenAbsent(t *testing.T) {
	root := t.TempDir()
	a := DirAction(root, "subdir/nested")
	if a.Kind != KindMkdir {
		t.Errorf("Kind = %q; want %q", a.Kind, KindMkdir)
	}
}

func TestDirAction_SkipWhenExists(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "exists"), 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}
	a := DirAction(root, "exists")
	if a.Kind != KindSkip {
		t.Errorf("Kind = %q; want %q", a.Kind, KindSkip)
	}
}

func TestExecute_WriteAndMkdir(t *testing.T) {
	root := t.TempDir()

	mk := DirAction(root, "made/sub")
	if err := Execute(&mk); err != nil {
		t.Fatalf("execute mkdir: %v", err)
	}
	if info, err := os.Stat(filepath.Join(root, "made", "sub")); err != nil || !info.IsDir() {
		t.Fatalf("mkdir did not create dir: %v", err)
	}

	wr := FileAction(root, "made/file.txt", []byte("body"), 0o644, false)
	if err := Execute(&wr); err != nil {
		t.Fatalf("execute write: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "made", "file.txt"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "body" {
		t.Errorf("file body = %q; want %q", got, "body")
	}
}

func TestExecute_SkipIsNoOp(t *testing.T) {
	a := Action{Kind: KindSkip, Path: "/nonexistent/should/not/be/touched"}
	if err := Execute(&a); err != nil {
		t.Errorf("skip should be no-op, got error: %v", err)
	}
}

func TestPrintPlan_LabelAndCounts(t *testing.T) {
	actions := []Action{
		{Rel: "a.txt", Kind: KindWriteFile, Reason: "create"},
		{Rel: "b.txt", Kind: KindSkip, Reason: "already up to date"},
		{Rel: "c/", Kind: KindMkdir, Reason: "create directory"},
	}
	var buf bytes.Buffer
	PrintPlan(&buf, actions, true, "docops upgrade")
	out := buf.String()
	if !strings.Contains(out, "docops upgrade: would apply 2 change(s), skipped 1") {
		t.Errorf("summary line missing/wrong: %s", out)
	}
	if !strings.Contains(out, "+ write-file") {
		t.Errorf("dry-run sigil missing for non-skip action: %s", out)
	}
	if !strings.Contains(out, "skip") {
		t.Errorf("skip line missing: %s", out)
	}
}

func TestLoadShippedSkills_NonEmpty(t *testing.T) {
	skills, err := LoadShippedSkills()
	if err != nil {
		t.Fatalf("LoadShippedSkills: %v", err)
	}
	if len(skills) == 0 {
		t.Fatal("expected at least one shipped skill")
	}
	for name, body := range skills {
		if !strings.HasSuffix(name, ".md") {
			t.Errorf("skill name %q does not end in .md", name)
		}
		if len(body) == 0 {
			t.Errorf("skill %q has empty body", name)
		}
	}
}
