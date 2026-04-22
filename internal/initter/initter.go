// Package initter implements `docops init`: turn a bare repo into a
// DocOps-enabled one. Safe to run twice (idempotent); --force overwrites
// drifted scaffolded files; --dry-run reports what would change without
// writing. The package is separated from cmd/docops so tests can drive
// it against a tempdir without spawning a subprocess.
package initter

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nachiket/docops/internal/config"
	"github.com/nachiket/docops/internal/schema"
	"github.com/nachiket/docops/templates"
)

// Options configures an init run. Root is the destination repository
// (typically cwd); the other flags mirror the CLI surface.
type Options struct {
	Root    string
	DryRun  bool
	Force   bool
	Out     io.Writer // human-readable progress; defaults to os.Stdout
	Verbose bool
}

// Action is a single filesystem change proposed by the planner. Init
// plans the full change set first, then executes it in one pass so
// --dry-run and the real write share the same code path.
type Action struct {
	// Path is the absolute destination.
	Path string

	// Rel is Path relative to Options.Root, used in human output.
	Rel string

	// Kind is one of "mkdir", "write-file", "merge-agents", "skip".
	Kind string

	// Reason explains why the action is a skip, write, or overwrite.
	Reason string

	// Body is the bytes that would be written. Empty for mkdir / skip.
	Body []byte

	// Mode is the permission for written files (0o755 for the hook, 0o644 otherwise).
	Mode os.FileMode
}

// Result is what Run returns after a plan execution.
type Result struct {
	Actions []Action
}

// Run plans and (unless --dry-run) executes the scaffolding. Returns a
// Result that lists every action — useful for assertions and for the
// "next steps" summary printed by cmd_init.
func Run(opts Options) (*Result, error) {
	if opts.Root == "" {
		return nil, fmt.Errorf("initter: Root must be set")
	}
	abs, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("initter: resolve root: %w", err)
	}
	opts.Root = abs
	if opts.Out == nil {
		opts.Out = os.Stdout
	}

	actions, err := plan(opts)
	if err != nil {
		return nil, err
	}

	if opts.DryRun {
		printPlan(opts.Out, actions, true)
		return &Result{Actions: actions}, nil
	}

	for i := range actions {
		if err := execute(&actions[i]); err != nil {
			return nil, fmt.Errorf("apply %s: %w", actions[i].Rel, err)
		}
	}
	printPlan(opts.Out, actions, false)
	return &Result{Actions: actions}, nil
}

// plan assembles the full list of scaffolding actions. Nothing writes
// to disk in here — it only reads existing files to decide whether each
// proposed target would be a create, a merge, or a skip.
func plan(opts Options) ([]Action, error) {
	cfg := config.Default()

	var actions []Action

	// 1. Directories.
	for _, rel := range []string{
		cfg.Paths.Context,
		cfg.Paths.Decisions,
		cfg.Paths.Tasks,
		cfg.Paths.Schema,
	} {
		actions = append(actions, dirAction(opts, rel))
	}

	// 2. docops.yaml at repo root.
	yamlBody, err := templates.DocopsYAML()
	if err != nil {
		return nil, fmt.Errorf("read docops.yaml template: %w", err)
	}
	actions = append(actions, fileAction(opts, "docops.yaml", yamlBody, 0o644))

	// 3. JSON Schema files.
	schemas, err := schema.JSONSchemas(schema.Config{ContextTypes: cfg.ContextTypes})
	if err != nil {
		return nil, fmt.Errorf("emit json schema: %w", err)
	}
	schemaNames := make([]string, 0, len(schemas))
	for name := range schemas {
		schemaNames = append(schemaNames, name)
	}
	sort.Strings(schemaNames)
	for _, name := range schemaNames {
		rel := filepath.Join(cfg.Paths.Schema, name)
		actions = append(actions, fileAction(opts, rel, schemas[name], 0o644))
	}

	// 4. AGENTS.md — delimited-block merge if a user file exists.
	agentsTmpl, err := templates.AgentsBlock()
	if err != nil {
		return nil, fmt.Errorf("read agents template: %w", err)
	}
	agentsAction, err := planAgents(opts, agentsTmpl)
	if err != nil {
		return nil, err
	}
	actions = append(actions, agentsAction)

	// 5. Pre-commit hook.
	hook, err := templates.PreCommitHook()
	if err != nil {
		return nil, fmt.Errorf("read pre-commit template: %w", err)
	}
	hookAction, err := planHook(opts, hook)
	if err != nil {
		return nil, err
	}
	actions = append(actions, hookAction)

	// 6. Agent skills — .claude/skills/docops/ and .cursor/commands/docops/.
	skills, err := templates.Skills()
	if err != nil {
		return nil, fmt.Errorf("read skills: %w", err)
	}
	skillNames := make([]string, 0, len(skills))
	for name := range skills {
		skillNames = append(skillNames, name)
	}
	sort.Strings(skillNames)
	for _, dir := range []string{".claude/skills/docops", ".cursor/commands/docops"} {
		actions = append(actions, dirAction(opts, dir))
		for _, name := range skillNames {
			rel := filepath.Join(dir, name)
			actions = append(actions, fileAction(opts, rel, skills[name], 0o644))
		}
	}

	return actions, nil
}

// dirAction builds a mkdir action that is a skip when the directory
// already exists. Keeps idempotent re-runs quiet.
func dirAction(opts Options, rel string) Action {
	abs := filepath.Join(opts.Root, rel)
	if info, err := os.Stat(abs); err == nil && info.IsDir() {
		return Action{
			Path:   abs,
			Rel:    rel,
			Kind:   "skip",
			Reason: "directory exists",
			Mode:   0o755,
		}
	}
	return Action{
		Path:   abs,
		Rel:    rel,
		Kind:   "mkdir",
		Reason: "create directory",
		Mode:   0o755,
	}
}

// fileAction builds a write-file action, deciding whether the target
// should be created, overwritten (--force on drift), or skipped.
func fileAction(opts Options, rel string, body []byte, mode os.FileMode) Action {
	abs := filepath.Join(opts.Root, rel)
	a := Action{Path: abs, Rel: rel, Kind: "write-file", Body: body, Mode: mode}
	existing, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			a.Reason = "create"
			return a
		}
		// Permission/other error — propagate via execute().
		a.Reason = "create (read failed: " + err.Error() + ")"
		return a
	}
	if bytes.Equal(existing, body) {
		a.Kind = "skip"
		a.Reason = "already up to date"
		return a
	}
	if opts.Force {
		a.Reason = "overwrite drifted content (--force)"
		return a
	}
	a.Kind = "skip"
	a.Reason = "exists and differs — rerun with --force to overwrite"
	return a
}

// planAgents decides how to render AGENTS.md. If the file is absent we
// write the template verbatim. If it exists with a block, we replace
// just the block. If it exists without a block, we append the block.
func planAgents(opts Options, tmpl []byte) (Action, error) {
	rel := "AGENTS.md"
	abs := filepath.Join(opts.Root, rel)

	existing, err := os.ReadFile(abs)
	if err != nil && !os.IsNotExist(err) {
		return Action{}, err
	}
	if os.IsNotExist(err) {
		return Action{
			Path:   abs,
			Rel:    rel,
			Kind:   "write-file",
			Body:   tmpl,
			Mode:   0o644,
			Reason: "create",
		}, nil
	}

	merged, changed, reason := mergeAgentsBlock(existing, tmpl)
	if !changed {
		return Action{
			Path:   abs,
			Rel:    rel,
			Kind:   "skip",
			Reason: reason,
		}, nil
	}
	return Action{
		Path:   abs,
		Rel:    rel,
		Kind:   "merge-agents",
		Body:   merged,
		Mode:   0o644,
		Reason: reason,
	}, nil
}

// planHook installs .git/hooks/pre-commit if .git exists. Otherwise it
// surfaces a skip reason so users with non-git workflows are not blocked.
func planHook(opts Options, body []byte) (Action, error) {
	gitDir := filepath.Join(opts.Root, ".git")
	info, err := os.Stat(gitDir)
	if err != nil || !info.IsDir() {
		return Action{
			Path:   filepath.Join(gitDir, "hooks", "pre-commit"),
			Rel:    ".git/hooks/pre-commit",
			Kind:   "skip",
			Reason: "no .git directory — run `git init` first or install the hook manually from templates/hooks/pre-commit",
		}, nil
	}
	rel := ".git/hooks/pre-commit"
	abs := filepath.Join(opts.Root, rel)
	a := Action{Path: abs, Rel: rel, Kind: "write-file", Body: body, Mode: 0o755}
	existing, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			a.Reason = "install"
			return a, nil
		}
		return Action{}, err
	}
	if bytes.Equal(existing, body) {
		a.Kind = "skip"
		a.Reason = "already up to date"
		return a, nil
	}
	if opts.Force {
		a.Reason = "overwrite drifted hook (--force)"
		return a, nil
	}
	a.Kind = "skip"
	a.Reason = "existing pre-commit hook differs — rerun with --force to overwrite"
	return a, nil
}

// blockStart and blockEnd match the delimiters defined in docops.yaml
// agents_md block_start / block_end. Keeping them as constants here
// mirrors the template default; a user that changes delimiters in
// docops.yaml will still get a correct write from init (no block exists
// yet) and can update via a future `docops refresh-agents-md` command.
const (
	blockStart = "<!-- docops:start -->"
	blockEnd   = "<!-- docops:end -->"
)

// mergeAgentsBlock replaces the docops:start/end block inside existing
// content with the block extracted from the template. If no block is
// present in existing, the template body is appended. Returns the merged
// bytes, a changed flag (false = identical to existing), and a reason
// string for human output.
func mergeAgentsBlock(existing, tmpl []byte) ([]byte, bool, string) {
	tmplBlock := extractBlock(tmpl)
	if tmplBlock == "" {
		// Template is malformed — return existing unchanged rather than
		// scribbling on the user's file.
		return existing, false, "template missing docops block; skipping merge"
	}

	ex := string(existing)
	startIdx := strings.Index(ex, blockStart)
	endIdx := strings.Index(ex, blockEnd)
	if startIdx >= 0 && endIdx > startIdx {
		endIdx += len(blockEnd)
		replacement := blockStart + tmplBlock + blockEnd
		merged := ex[:startIdx] + replacement + ex[endIdx:]
		if merged == ex {
			return existing, false, "docops block already up to date"
		}
		return []byte(merged), true, "refresh docops block"
	}

	// No block yet — append the template block (with delimiters) to the end.
	appended := ex
	if !strings.HasSuffix(appended, "\n") {
		appended += "\n"
	}
	appended += "\n" + blockStart + tmplBlock + blockEnd + "\n"
	return []byte(appended), true, "append docops block to existing AGENTS.md"
}

// extractBlock returns the content between blockStart and blockEnd in
// tmpl. Returns "" if either marker is missing. Leading/trailing
// newlines are preserved so the rendered block is visually identical
// to the template.
func extractBlock(tmpl []byte) string {
	s := string(tmpl)
	start := strings.Index(s, blockStart)
	if start < 0 {
		return ""
	}
	start += len(blockStart)
	end := strings.Index(s, blockEnd)
	if end < 0 || end < start {
		return ""
	}
	return s[start:end]
}

// execute applies one planned action to the filesystem.
func execute(a *Action) error {
	switch a.Kind {
	case "skip":
		return nil
	case "mkdir":
		return os.MkdirAll(a.Path, 0o755)
	case "write-file", "merge-agents":
		if err := os.MkdirAll(filepath.Dir(a.Path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(a.Path, a.Body, a.Mode); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("unknown action kind %q", a.Kind)
}

// printPlan writes a human-readable summary of the action set. For
// dry-run we use the "would" phrasing; for real runs we use past tense.
func printPlan(w io.Writer, actions []Action, dry bool) {
	var changed, skipped int
	for _, a := range actions {
		if a.Kind == "skip" {
			skipped++
		} else {
			changed++
		}
	}
	verb := "applied"
	if dry {
		verb = "would apply"
	}
	fmt.Fprintf(w, "docops init: %s %d change(s), skipped %d\n", verb, changed, skipped)
	for _, a := range actions {
		tag := a.Kind
		if dry && a.Kind != "skip" {
			tag = "+ " + a.Kind
		}
		fmt.Fprintf(w, "  %-13s %-34s %s\n", tag, a.Rel, a.Reason)
	}
	if !dry && changed > 0 {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "next steps:")
		fmt.Fprintln(w, "  docops validate           # confirm the scaffolded docs parse")
		fmt.Fprintln(w, "  docops new ctx \"…\" --type brief")
		fmt.Fprintln(w, "  docops new adr \"…\"")
	}
}
