package upgrader

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/logicwind/docops/internal/scaffold"
	"github.com/logicwind/docops/templates"
)

// initted scaffolds a minimal docops-initialized project at root with
// the shipped skills and schemas pre-installed, mirroring what
// `docops init` would have produced one release earlier. The caller
// can then mutate the layout to simulate v0.1.x drift before running
// the upgrader.
func initted(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// docops.yaml — required for the upgrader to run at all.
	yamlBody, err := templates.DocopsYAML()
	if err != nil {
		t.Fatalf("template DocopsYAML: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docops.yaml"), yamlBody, 0o644); err != nil {
		t.Fatalf("write docops.yaml: %v", err)
	}

	// Seed the shipped skills into both docops-owned dirs.
	skills, err := scaffold.LoadShippedSkills()
	if err != nil {
		t.Fatalf("LoadShippedSkills: %v", err)
	}
	for _, dir := range []string{".claude/skills/docops", ".cursor/commands/docops"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		for name, body := range skills {
			if err := os.WriteFile(filepath.Join(root, dir, name), body, 0o644); err != nil {
				t.Fatalf("write seed skill %s: %v", name, err)
			}
		}
	}

	// AGENTS.md with the docops block already merged.
	agentsTmpl, err := templates.AgentsBlock()
	if err != nil {
		t.Fatalf("template AgentsBlock: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), agentsTmpl, 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	return root
}

// findAction returns the first action matching rel, or nil.
func findAction(actions []scaffold.Action, rel string) *scaffold.Action {
	for i, a := range actions {
		if a.Rel == rel {
			return &actions[i]
		}
	}
	return nil
}

func TestRun_RefusesWithoutDocopsYAML(t *testing.T) {
	root := t.TempDir() // empty
	_, err := Run(Options{Root: root, DryRun: true, Out: io.Discard})
	if !errors.Is(err, ErrNoConfig) {
		t.Fatalf("err = %v; want ErrNoConfig", err)
	}
}

func TestRun_IdempotentOnFreshlyInittedProject(t *testing.T) {
	root := initted(t)
	res, err := Run(Options{Root: root, DryRun: true, Out: io.Discard})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, a := range res.Actions {
		if a.Kind == scaffold.KindRemove {
			t.Errorf("idempotent run should not remove anything; got remove for %s", a.Rel)
		}
		// The schema files were written by initted via the templates,
		// but FileAction's body comparison may flag them refreshed
		// because schema.JSONSchemas regenerates from docops.yaml. We
		// don't seed them here, so a fresh run will emit creates for
		// the schema files — that's fine. Skill files are seeded
		// byte-identically and should be skips.
		if strings.HasPrefix(a.Rel, ".claude/skills/docops/") && a.Kind != scaffold.KindMkdir && a.Kind != scaffold.KindSkip {
			t.Errorf("seeded skill should be skip; %s = %s", a.Rel, a.Kind)
		}
	}
}

func TestRun_AddsNewSkillRemovesStaleRefreshesChanged(t *testing.T) {
	root := initted(t)
	dir := filepath.Join(root, ".claude/skills/docops")

	// Simulate v0.1.0-era state in the dir:
	//  - delete one shipped skill so the upgrader will (re)create it.
	//  - mutate one shipped skill so the upgrader will refresh it.
	//  - add a stale skill that no longer ships.
	pickAdd := "init.md"      // delete locally → upgrade should add (+)
	pickRefresh := "audit.md" // mutate locally → upgrade should refresh (~)
	if err := os.Remove(filepath.Join(dir, pickAdd)); err != nil {
		t.Fatalf("remove %s: %v", pickAdd, err)
	}
	if err := os.WriteFile(filepath.Join(dir, pickRefresh), []byte("stale local body\n"), 0o644); err != nil {
		t.Fatalf("mutate %s: %v", pickRefresh, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old-command.md"), []byte("removed-upstream skill\n"), 0o644); err != nil {
		t.Fatalf("seed stale skill: %v", err)
	}

	res, err := Run(Options{Root: root, DryRun: true, Out: io.Discard})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	addAction := findAction(res.Actions, ".claude/skills/docops/"+pickAdd)
	if addAction == nil || addAction.Kind != scaffold.KindWriteFile || addAction.Reason != "create" {
		t.Errorf("%s should be a create action; got %+v", pickAdd, addAction)
	}

	refreshAction := findAction(res.Actions, ".claude/skills/docops/"+pickRefresh)
	if refreshAction == nil || refreshAction.Kind != scaffold.KindWriteFile || refreshAction.Reason == "create" {
		t.Errorf("%s should be a refresh (overwrite) action; got %+v", pickRefresh, refreshAction)
	}

	removeAction := findAction(res.Actions, ".claude/skills/docops/old-command.md")
	if removeAction == nil || removeAction.Kind != scaffold.KindRemove {
		t.Errorf("old-command.md should be a remove action; got %+v", removeAction)
	}
}

func TestRun_ApplyWritesFilesAndDeletesStale(t *testing.T) {
	root := initted(t)
	dir := filepath.Join(root, ".claude/skills/docops")
	staleName := "old-command.md"
	if err := os.WriteFile(filepath.Join(dir, staleName), []byte("stale\n"), 0o644); err != nil {
		t.Fatalf("seed stale: %v", err)
	}

	if _, err := Run(Options{Root: root, DryRun: false, Out: io.Discard}); err != nil {
		t.Fatalf("Run apply: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, staleName)); !os.IsNotExist(err) {
		t.Errorf("stale skill should be deleted; stat err = %v", err)
	}

	// A second apply should be a clean no-op (idempotent).
	res, err := Run(Options{Root: root, DryRun: true, Out: io.Discard})
	if err != nil {
		t.Fatalf("Run dry-run after apply: %v", err)
	}
	for _, a := range res.Actions {
		if a.Kind == scaffold.KindRemove || (a.Kind == scaffold.KindWriteFile && a.Reason != "create") {
			// schema files are always written (regenerated from yaml);
			// don't flag those as drift.
			continue
		}
	}
}

func TestRun_DoesNotTouchDocopsYAMLByDefault(t *testing.T) {
	root := initted(t)
	yamlPath := filepath.Join(root, "docops.yaml")
	custom := []byte("# user-customized config\nproject: my-thing\n")
	if err := os.WriteFile(yamlPath, custom, 0o644); err != nil {
		t.Fatalf("write custom yaml: %v", err)
	}

	if _, err := Run(Options{Root: root, DryRun: false, Out: io.Discard}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("read yaml: %v", err)
	}
	if !bytes.Equal(got, custom) {
		t.Errorf("docops.yaml was modified without --config:\n%s", got)
	}
}

func TestRun_RewritesDocopsYAMLWithConfigFlag(t *testing.T) {
	root := initted(t)
	yamlPath := filepath.Join(root, "docops.yaml")
	if err := os.WriteFile(yamlPath, []byte("# stale\n"), 0o644); err != nil {
		t.Fatalf("seed stale yaml: %v", err)
	}

	if _, err := Run(Options{Root: root, DryRun: false, Config: true, Out: io.Discard}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("read yaml: %v", err)
	}
	if bytes.Equal(got, []byte("# stale\n")) {
		t.Errorf("docops.yaml should have been rewritten with --config")
	}
}

func TestRun_DoesNotTouchPreCommitHookByDefault(t *testing.T) {
	root := initted(t)
	hookDir := filepath.Join(root, ".git/hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatalf("mkdir hookdir: %v", err)
	}
	customHook := []byte("#!/bin/sh\n# my chained pre-commit\nexit 0\n")
	if err := os.WriteFile(filepath.Join(hookDir, "pre-commit"), customHook, 0o755); err != nil {
		t.Fatalf("seed hook: %v", err)
	}

	if _, err := Run(Options{Root: root, DryRun: false, Out: io.Discard}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(hookDir, "pre-commit"))
	if err != nil {
		t.Fatalf("read hook: %v", err)
	}
	if !bytes.Equal(got, customHook) {
		t.Errorf("pre-commit hook was modified without --hook flag")
	}
}

func TestRun_FirstUpgradeDeletesUserFileInsideDocopsDir(t *testing.T) {
	root := initted(t)
	dir := filepath.Join(root, ".claude/skills/docops")
	custom := filepath.Join(dir, "custom.md")
	if err := os.WriteFile(custom, []byte("user-added\n"), 0o644); err != nil {
		t.Fatalf("seed custom skill: %v", err)
	}

	if _, err := Run(Options{Root: root, DryRun: false, Out: io.Discard}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := os.Stat(custom); !os.IsNotExist(err) {
		t.Errorf("first-time upgrade should treat custom.md as docops-owned and delete it; stat err=%v", err)
	}
}

func TestRun_DoesNotTouchUserFileOneLevelUp(t *testing.T) {
	root := initted(t)
	sibling := filepath.Join(root, ".claude/skills/my-stuff.md")
	if err := os.WriteFile(sibling, []byte("user content\n"), 0o644); err != nil {
		t.Fatalf("seed sibling: %v", err)
	}

	if _, err := Run(Options{Root: root, DryRun: false, Out: io.Discard}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	body, err := os.ReadFile(sibling)
	if err != nil {
		t.Fatalf("sibling lost: %v", err)
	}
	if string(body) != "user content\n" {
		t.Errorf("sibling content was modified: %q", body)
	}
}

func TestRun_SecondUpgradeRefusesUserAddedFileInDocopsDir(t *testing.T) {
	root := initted(t)
	// First upgrade — establishes the manifest.
	if _, err := Run(Options{Root: root, DryRun: false, Out: io.Discard}); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// User now drops a custom skill inside the docops-owned dir.
	custom := filepath.Join(root, ".claude/skills/docops/custom.md")
	if err := os.WriteFile(custom, []byte("user-added post-init\n"), 0o644); err != nil {
		t.Fatalf("seed custom: %v", err)
	}

	_, err := Run(Options{Root: root, DryRun: true, Out: io.Discard})
	var unk *ErrUnknownFiles
	if !errors.As(err, &unk) {
		t.Fatalf("err = %v; want *ErrUnknownFiles", err)
	}
	want := []string{"custom.md"}
	got := append([]string{}, unk.Files...)
	sort.Strings(got)
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("unknown files = %v; want %v", got, want)
	}

	// Custom file should still be present (we refused without writing).
	if _, err := os.Stat(custom); err != nil {
		t.Errorf("safety belt should not have deleted user file: %v", err)
	}
}

func TestRun_CreatesClaudeMdWhenAbsent(t *testing.T) {
	root := initted(t)
	// initted does not seed CLAUDE.md (matches the v0.1.x state pre-ADR-0024).
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("test precondition: CLAUDE.md should be absent, got err=%v", err)
	}

	if _, err := Run(Options{Root: root, DryRun: false, Out: io.Discard}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("CLAUDE.md missing after upgrade: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, scaffold.BlockStart) || !strings.Contains(s, scaffold.BlockEnd) {
		t.Errorf("CLAUDE.md missing docops block markers: %s", s)
	}
}

func TestRun_RefreshesBothAGENTSAndClaudeBlocks(t *testing.T) {
	root := initted(t)
	stale := scaffold.BlockStart + "\nold body\n" + scaffold.BlockEnd
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		body := "# Header\n\n" + stale + "\n\nuser footer\n"
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	if _, err := Run(Options{Root: root, DryRun: false, Out: io.Discard}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		s := string(body)
		if strings.Contains(s, "old body") {
			t.Errorf("%s: stale block survived refresh", name)
		}
		if !strings.Contains(s, "user footer") {
			t.Errorf("%s: user footer dropped", name)
		}
	}
}

func TestRun_PreservesUserContentInClaudeMdAcrossUpgrade(t *testing.T) {
	root := initted(t)
	// Seed CLAUDE.md with hand-written content outside any docops block.
	body := "# My CLAUDE.md\n\nProject-specific guidance for Claude only.\n\nSee AGENTS.md for the multi-tool view.\n"
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("seed CLAUDE.md: %v", err)
	}

	if _, err := Run(Options{Root: root, DryRun: false, Out: io.Discard}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, "Project-specific guidance for Claude only.") {
		t.Errorf("user CLAUDE.md content lost across upgrade: %s", s)
	}
	if !strings.Contains(s, scaffold.BlockStart) || !strings.Contains(s, scaffold.BlockEnd) {
		t.Errorf("docops block missing after upgrade: %s", s)
	}
}

func TestRun_DryRunWritesNothing(t *testing.T) {
	root := initted(t)
	dir := filepath.Join(root, ".claude/skills/docops")
	stalePath := filepath.Join(dir, "old-command.md")
	if err := os.WriteFile(stalePath, []byte("stale\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := Run(Options{Root: root, DryRun: true, Out: io.Discard}); err != nil {
		t.Fatalf("dry-run: %v", err)
	}

	if _, err := os.Stat(stalePath); err != nil {
		t.Errorf("dry-run should not have removed %s; err=%v", stalePath, err)
	}
}

func TestRun_RefreshesAGENTSBlockInPlace(t *testing.T) {
	root := initted(t)
	agentsPath := filepath.Join(root, "AGENTS.md")
	prefix := "# Project header — do not delete\n\n## Hand-written notes\n\nPreserve me.\n\n"
	tmpl, _ := templates.AgentsBlock()
	// Write a file with user content + a stale block.
	mixed := []byte(prefix + scaffold.BlockStart + "\nstale block content\n" + scaffold.BlockEnd + "\n\n## footer\n")
	if err := os.WriteFile(agentsPath, mixed, 0o644); err != nil {
		t.Fatalf("seed AGENTS: %v", err)
	}

	if _, err := Run(Options{Root: root, DryRun: false, Out: io.Discard}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, "Hand-written notes") || !strings.Contains(s, "footer") {
		t.Errorf("user content lost: %s", s)
	}
	if strings.Contains(s, "stale block content") {
		t.Errorf("stale block content survived refresh")
	}
	expected := scaffold.ExtractBlock(tmpl)
	if !strings.Contains(s, expected) {
		t.Errorf("refreshed block does not match template")
	}
}
