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

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/scaffold"
	"github.com/logicwind/docops/internal/schema"
	"github.com/logicwind/docops/templates"
)

// Options configures an init run. Root is the destination repository
// (typically cwd); the other flags mirror the CLI surface.
type Options struct {
	Root     string
	DryRun   bool
	Force    bool
	NoSkills bool      // skip scaffolding .claude/commands/docops/ and .cursor/commands/docops/
	Out      io.Writer // human-readable progress; defaults to os.Stdout
	Verbose  bool
}

// Action aliases scaffold.Action so existing initter consumers keep
// their imports unchanged after the scaffold extraction (TP-020).
type Action = scaffold.Action

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
		if err := scaffold.Execute(&actions[i]); err != nil {
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
	if loaded, err := config.Load(filepath.Join(opts.Root, config.DefaultFilename)); err == nil {
		cfg = loaded
	}

	var actions []Action

	// 1. Directories.
	for _, rel := range []string{
		cfg.Paths.Context,
		cfg.Paths.Decisions,
		cfg.Paths.Tasks,
		cfg.Paths.Schema,
	} {
		actions = append(actions, scaffold.DirAction(opts.Root, rel))
	}

	// 2. docops.yaml at repo root.
	yamlBody, err := templates.DocopsYAML()
	if err != nil {
		return nil, fmt.Errorf("read docops.yaml template: %w", err)
	}
	actions = append(actions, scaffold.FileAction(opts.Root, "docops.yaml", yamlBody, 0o644, opts.Force))

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
		actions = append(actions, scaffold.FileAction(opts.Root, rel, schemas[name], 0o644, opts.Force))
	}

	// 4. AGENTS.md and CLAUDE.md — delimited-block merge if a user
	// file exists, otherwise write the template verbatim. Both files
	// are docops-managed and share the same docops block (ADR-0024).
	agentsTmpl, err := templates.AgentsBlock()
	if err != nil {
		return nil, fmt.Errorf("read agents template: %w", err)
	}
	agentsAction, err := planMarkdownBlock(opts, "AGENTS.md", agentsTmpl)
	if err != nil {
		return nil, err
	}
	actions = append(actions, agentsAction)

	claudeTmpl, err := templates.ClaudeBlock()
	if err != nil {
		return nil, fmt.Errorf("read claude template: %w", err)
	}
	claudeAction, err := planMarkdownBlock(opts, "CLAUDE.md", claudeTmpl)
	if err != nil {
		return nil, err
	}
	actions = append(actions, claudeAction)

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

	// 6. Agent slash commands — .claude/commands/docops/ and
	// .cursor/commands/docops/. Per ADR-0029, only the milestone-moment
	// subset (SlashDeliverableCmds) ships as slashes; the full shipped
	// set still lives as skills (NL-dispatched) and CLI verbs, so the
	// granular surface is reachable without a slash. Skipped entirely
	// when --no-skills is set; existing files are not touched.
	if !opts.NoSkills {
		skills, err := scaffold.LoadShippedSkills()
		if err != nil {
			return nil, fmt.Errorf("read skills: %w", err)
		}
		keep := scaffold.SlashDeliverableCmds()
		skillNames := make([]string, 0, len(skills))
		for name := range skills {
			cmd := name
			if i := len(name) - len(".md"); i > 0 && name[i:] == ".md" {
				cmd = name[:i]
			}
			if !keep[cmd] {
				continue
			}
			skillNames = append(skillNames, name)
		}
		sort.Strings(skillNames)
		for _, dir := range []string{".claude/commands/docops", ".cursor/commands/docops"} {
			actions = append(actions, scaffold.DirAction(opts.Root, dir))
			for _, name := range skillNames {
				rel := filepath.Join(dir, name)
				actions = append(actions, scaffold.FileAction(opts.Root, rel, skills[name], 0o644, opts.Force))
			}
		}
	}

	return actions, nil
}

// planMarkdownBlock decides how to render a docops-managed markdown
// file (AGENTS.md, CLAUDE.md). If the file is absent we write the
// template verbatim. If it exists with a docops block, we refresh
// just the block. If it exists without a block, we append the block.
// Used by both init and (indirectly) upgrade via the same logic.
func planMarkdownBlock(opts Options, rel string, tmpl []byte) (Action, error) {
	abs := filepath.Join(opts.Root, rel)

	existing, err := os.ReadFile(abs)
	if err != nil && !os.IsNotExist(err) {
		return Action{}, err
	}
	if os.IsNotExist(err) {
		return Action{
			Path:   abs,
			Rel:    rel,
			Kind:   scaffold.KindWriteFile,
			Body:   tmpl,
			Mode:   0o644,
			Reason: "create",
		}, nil
	}

	merged, changed, reason := scaffold.MergeAgentsBlock(existing, tmpl)
	if !changed {
		return Action{
			Path:   abs,
			Rel:    rel,
			Kind:   scaffold.KindSkip,
			Reason: reason,
		}, nil
	}
	return Action{
		Path:   abs,
		Rel:    rel,
		Kind:   scaffold.KindMergeAgents,
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
			Kind:   scaffold.KindSkip,
			Reason: "no .git directory — run `git init` first or install the hook manually from templates/hooks/pre-commit",
		}, nil
	}
	rel := ".git/hooks/pre-commit"
	abs := filepath.Join(opts.Root, rel)
	a := Action{Path: abs, Rel: rel, Kind: scaffold.KindWriteFile, Body: body, Mode: 0o755}
	existing, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			a.Reason = "install"
			return a, nil
		}
		return Action{}, err
	}
	if bytes.Equal(existing, body) {
		a.Kind = scaffold.KindSkip
		a.Reason = "already up to date"
		return a, nil
	}
	if opts.Force {
		a.Reason = "overwrite drifted hook (--force)"
		return a, nil
	}
	a.Kind = scaffold.KindSkip
	a.Reason = "existing pre-commit hook differs — rerun with --force to overwrite"
	return a, nil
}

// printPlan emits the scaffold action summary. The closing next-step
// block is rendered by the CLI layer (cmd_init) via the nextsteps
// package, which can route greenfield vs brownfield.
func printPlan(w io.Writer, actions []Action, dry bool) {
	scaffold.PrintPlan(w, actions, dry, "docops init")
}
