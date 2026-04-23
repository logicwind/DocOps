package initter

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/logicwind/docops/internal/scaffold"
)

// withGit creates a fake .git directory so planHook does not short-circuit.
func withGit(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatalf("mkdir .git/hooks: %v", err)
	}
}

func TestRun_BareRepo_CreatesAllArtifacts(t *testing.T) {
	root := t.TempDir()
	withGit(t, root)

	res, err := Run(Options{Root: root, Out: io.Discard})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Every scaffold target must exist.
	for _, rel := range []string{
		"docs/context",
		"docs/decisions",
		"docs/tasks",
		"docs/.docops/schema",
		"docs/.docops/schema/context.schema.json",
		"docs/.docops/schema/decision.schema.json",
		"docs/.docops/schema/task.schema.json",
		"docops.yaml",
		"AGENTS.md",
		".git/hooks/pre-commit",
		".claude/commands/docops/init.md",
		".claude/commands/docops/state.md",
		".claude/commands/docops/new-ctx.md",
		".cursor/commands/docops/init.md",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("expected %s: %v", rel, err)
		}
	}

	// Pre-commit hook should be executable on unix. Windows does not
	// expose a POSIX executable bit, so skip the permission assertion.
	info, err := os.Stat(filepath.Join(root, ".git/hooks/pre-commit"))
	if err != nil {
		t.Fatalf("stat hook: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o100 == 0 {
		t.Errorf("pre-commit hook not executable: mode=%v", info.Mode())
	}

	// Result should include at least one "create" action per scaffolded file.
	creates := 0
	for _, a := range res.Actions {
		if a.Kind == "write-file" || a.Kind == "merge-agents" {
			creates++
		}
	}
	if creates < 10 {
		t.Errorf("expected ≥10 write actions, got %d", creates)
	}
}

func TestRun_Idempotent(t *testing.T) {
	root := t.TempDir()
	withGit(t, root)

	if _, err := Run(Options{Root: root, Out: io.Discard}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	res, err := Run(Options{Root: root, Out: io.Discard})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	// On a clean re-run, every file action should be a skip-because-up-to-date.
	for _, a := range res.Actions {
		switch a.Kind {
		case "write-file", "merge-agents":
			t.Errorf("second run wrote %s (%s) — expected skip", a.Rel, a.Reason)
		}
	}
}

func TestRun_Force_OverwritesDrift(t *testing.T) {
	root := t.TempDir()
	withGit(t, root)

	if _, err := Run(Options{Root: root, Out: io.Discard}); err != nil {
		t.Fatalf("setup run: %v", err)
	}

	// Simulate drift: user hand-edits docops.yaml.
	drifted := filepath.Join(root, "docops.yaml")
	if err := os.WriteFile(drifted, []byte("# hand-edited garbage\n"), 0o644); err != nil {
		t.Fatalf("write drift: %v", err)
	}

	// Without --force, init must refuse to overwrite.
	res, err := Run(Options{Root: root, Out: io.Discard})
	if err != nil {
		t.Fatalf("no-force run: %v", err)
	}
	if action := findAction(res.Actions, "docops.yaml"); action == nil || action.Kind != "skip" {
		t.Fatalf("no-force: expected skip for docops.yaml, got %+v", action)
	}
	body, _ := os.ReadFile(drifted)
	if !strings.Contains(string(body), "hand-edited garbage") {
		t.Errorf("no-force should have left drifted file alone, got %s", body)
	}

	// With --force, it overwrites.
	res, err = Run(Options{Root: root, Force: true, Out: io.Discard})
	if err != nil {
		t.Fatalf("force run: %v", err)
	}
	if action := findAction(res.Actions, "docops.yaml"); action == nil || action.Kind != "write-file" {
		t.Fatalf("force: expected write-file for docops.yaml, got %+v", action)
	}
	body, _ = os.ReadFile(drifted)
	if strings.Contains(string(body), "hand-edited garbage") {
		t.Errorf("force should have overwritten drift, got %s", body)
	}
}

func TestRun_DryRun_WritesNothing(t *testing.T) {
	root := t.TempDir()
	withGit(t, root)

	var buf bytes.Buffer
	if _, err := Run(Options{Root: root, DryRun: true, Out: &buf}); err != nil {
		t.Fatalf("dry-run: %v", err)
	}

	// docops.yaml should NOT exist.
	if _, err := os.Stat(filepath.Join(root, "docops.yaml")); !os.IsNotExist(err) {
		t.Errorf("dry-run created docops.yaml: err=%v", err)
	}
	// Output must mention "would apply".
	if !strings.Contains(buf.String(), "would apply") {
		t.Errorf("dry-run output lacks 'would apply': %s", buf.String())
	}
}

func TestRun_AgentsMerge_PreservesUserContent(t *testing.T) {
	root := t.TempDir()
	withGit(t, root)

	userBody := "# Existing AGENTS.md\n\nOwn content that DocOps must not touch.\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(userBody), 0o644); err != nil {
		t.Fatalf("write user agents: %v", err)
	}

	if _, err := Run(Options{Root: root, Out: io.Discard}); err != nil {
		t.Fatalf("run: %v", err)
	}

	merged, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read merged: %v", err)
	}
	s := string(merged)
	if !strings.Contains(s, "Own content that DocOps must not touch.") {
		t.Errorf("user content missing after merge: %s", s)
	}
	if !strings.Contains(s, scaffold.BlockStart) || !strings.Contains(s, scaffold.BlockEnd) {
		t.Errorf("docops block delimiters missing after merge: %s", s)
	}
}

func TestRun_AgentsBlockRefresh_ReplacesBlockOnly(t *testing.T) {
	root := t.TempDir()
	withGit(t, root)

	existing := "# Header\n\n" + scaffold.BlockStart + "\nstale block content\n" + scaffold.BlockEnd + "\n\n## Keep me\n\nuser footer\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	if _, err := Run(Options{Root: root, Out: io.Discard}); err != nil {
		t.Fatalf("run: %v", err)
	}

	after, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	s := string(after)
	if strings.Contains(s, "stale block content") {
		t.Errorf("stale block not replaced: %s", s)
	}
	if !strings.Contains(s, "user footer") {
		t.Errorf("user footer dropped: %s", s)
	}
	if !strings.Contains(s, "# Header") {
		t.Errorf("user header dropped: %s", s)
	}
}

func TestRun_CreatesClaudeMdAlongsideAgentsMd(t *testing.T) {
	root := t.TempDir()
	withGit(t, root)

	if _, err := Run(Options{Root: root, Out: io.Discard}); err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("%s missing after init: %v", name, err)
		}
		s := string(body)
		if !strings.Contains(s, scaffold.BlockStart) || !strings.Contains(s, scaffold.BlockEnd) {
			t.Errorf("%s missing docops block markers", name)
		}
	}
}

func TestRun_ClaudeMdMerge_PreservesUserContent(t *testing.T) {
	root := t.TempDir()
	withGit(t, root)

	userBody := "# Custom CLAUDE.md\n\nThis project's specific Claude tweaks.\n"
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(userBody), 0o644); err != nil {
		t.Fatalf("write user CLAUDE.md: %v", err)
	}

	if _, err := Run(Options{Root: root, Out: io.Discard}); err != nil {
		t.Fatalf("run: %v", err)
	}

	merged, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read merged: %v", err)
	}
	s := string(merged)
	if !strings.Contains(s, "This project's specific Claude tweaks.") {
		t.Errorf("user content lost: %s", s)
	}
	if !strings.Contains(s, scaffold.BlockStart) || !strings.Contains(s, scaffold.BlockEnd) {
		t.Errorf("docops block missing after merge: %s", s)
	}
}

func TestRun_NoGitDir_SkipsHook(t *testing.T) {
	root := t.TempDir()
	// no withGit()

	res, err := Run(Options{Root: root, Out: io.Discard})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	a := findAction(res.Actions, ".git/hooks/pre-commit")
	if a == nil {
		t.Fatal("missing pre-commit action")
	}
	if a.Kind != "skip" {
		t.Errorf("expected skip without .git, got %q (%s)", a.Kind, a.Reason)
	}
}

// TestRun_UsesProjectContextTypes verifies that when a docops.yaml with
// custom context_types exists at the root, init uses those types in the
// emitted context.schema.json rather than the built-in defaults.
func TestRun_UsesProjectContextTypes(t *testing.T) {
	root := t.TempDir()
	withGit(t, root)

	// Write a minimal docops.yaml with non-default context_types.
	yaml := "version: 1\ncontext_types: [alpha, beta]\n"
	if err := os.WriteFile(filepath.Join(root, "docops.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write docops.yaml: %v", err)
	}

	if _, err := Run(Options{Root: root, Force: true, Out: io.Discard}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(root, "docs/.docops/schema/context.schema.json"))
	if err != nil {
		t.Fatalf("read context.schema.json: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, `"alpha"`) || !strings.Contains(s, `"beta"`) {
		t.Errorf("context.schema.json missing custom context_types: %s", s)
	}
	// Default types must NOT appear (they were replaced, not merged).
	if strings.Contains(s, `"prd"`) {
		t.Errorf("context.schema.json still contains default type 'prd' after custom override: %s", s)
	}
}

// TestRun_NoSkills_SkipsSkillDirs verifies that --no-skills prevents
// creation of .claude/commands/docops/ and .cursor/commands/docops/.
func TestRun_NoSkills_SkipsSkillDirs(t *testing.T) {
	root := t.TempDir()
	withGit(t, root)

	res, err := Run(Options{Root: root, NoSkills: true, Out: io.Discard})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Neither skill directory nor any skill file should be created.
	for _, rel := range []string{
		".claude/commands/docops",
		".cursor/commands/docops",
	} {
		abs := filepath.Join(root, rel)
		if _, statErr := os.Stat(abs); statErr == nil {
			t.Errorf("--no-skills: %s was created but should not have been", rel)
		}
	}

	// No action in the result should reference those paths.
	for _, a := range res.Actions {
		if len(a.Rel) >= len(".claude") && a.Rel[:len(".claude")] == ".claude" {
			t.Errorf("--no-skills: action references .claude path: %s (kind=%s)", a.Rel, a.Kind)
		}
		if len(a.Rel) >= len(".cursor") && a.Rel[:len(".cursor")] == ".cursor" {
			t.Errorf("--no-skills: action references .cursor path: %s (kind=%s)", a.Rel, a.Kind)
		}
	}
}

func findAction(actions []Action, rel string) *Action {
	for i, a := range actions {
		if a.Rel == rel {
			return &actions[i]
		}
	}
	return nil
}
