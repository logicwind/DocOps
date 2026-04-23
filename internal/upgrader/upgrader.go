// Package upgrader implements `docops upgrade`: an in-band refresh of
// docops-owned scaffolding (skills, schemas, AGENTS.md block) for
// projects already initialized via `docops init`. The contract is
// narrower than init's: we never touch user-owned files
// (docops.yaml, pre-commit hook, docs/{context,decisions,tasks}/*)
// unless an opt-in flag asks us to. See ADR-0021 for the rationale.
package upgrader

import (
	"bytes"
	"errors"
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

// Options configures an upgrade run. Root must contain a docops.yaml
// (Run rejects the call otherwise so users do not accidentally
// scaffold a fresh project via the wrong subcommand).
type Options struct {
	Root   string
	DryRun bool
	// Config opts in to overwriting docops.yaml from the shipped template.
	Config bool
	// Hook opts in to reinstalling the pre-commit hook.
	Hook bool
	// Out is the human-readable progress sink; defaults to os.Stdout.
	Out io.Writer
}

// Result mirrors initter.Result: the planned/applied actions in order.
type Result struct {
	Actions []scaffold.Action
}

// ErrNoConfig signals the project is not docops-initialized yet. The
// CLI converts this into a clear "run docops init first" message and
// exits 2 (matching every other non-init bootstrap error).
var ErrNoConfig = errors.New("upgrader: no docops.yaml at root — run `docops init` first")

// ErrUnknownFiles signals the safety belt fired: the docops-owned
// skill directory contains files that neither the manifest nor the
// shipped bundle accounts for. The CLI prints the file list verbatim
// and exits 2 without touching anything.
type ErrUnknownFiles struct {
	Dir   string
	Files []string
}

func (e *ErrUnknownFiles) Error() string {
	return fmt.Sprintf("upgrader: %s contains user-added files not in the docops bundle: %v", e.Dir, e.Files)
}

// Run plans and (unless DryRun) executes the upgrade. Returns
// ErrNoConfig if the root has no docops.yaml; *ErrUnknownFiles if a
// docops-owned skill dir contains files outside the shipped bundle
// and the manifest.
func Run(opts Options) (*Result, error) {
	if opts.Root == "" {
		return nil, errors.New("upgrader: Root must be set")
	}
	abs, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("upgrader: resolve root: %w", err)
	}
	opts.Root = abs
	if opts.Out == nil {
		opts.Out = os.Stdout
	}

	cfgPath := filepath.Join(opts.Root, config.DefaultFilename)
	if _, err := os.Stat(cfgPath); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoConfig
		}
		return nil, fmt.Errorf("upgrader: stat %s: %w", config.DefaultFilename, err)
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
	// After successful execution, refresh the manifest in each
	// docops-owned skill dir so subsequent upgrades scope deletions
	// correctly. Failures here are logged but do not fail the upgrade
	// — manifest is best-effort metadata.
	for _, dir := range docopsSkillDirs() {
		if err := writeManifest(filepath.Join(opts.Root, dir), shippedSkillNames(actions, dir)); err != nil {
			fmt.Fprintf(opts.Out, "  warning: refresh manifest in %s: %v\n", dir, err)
		}
	}
	printPlan(opts.Out, actions, false)
	return &Result{Actions: actions}, nil
}

// docopsSkillDirs lists the directories upgrade owns. A future plugin
// system might extend this; for now it is fixed to the two
// agent-tool conventions docops init scaffolds.
func docopsSkillDirs() []string {
	return []string{".claude/commands/docops", ".cursor/commands/docops"}
}

// legacyDocopsSkillDirs lists paths that earlier docops releases wrote
// skill files into and that upgrade should actively clean up. Claude
// Code reads slash commands from .claude/commands/, not .claude/skills/,
// so v0.2.x wrote to the wrong folder; v0.3.x moves them.
func legacyDocopsSkillDirs() []string {
	return []string{".claude/skills/docops"}
}

// shippedSkillNames returns the basenames of skill files currently
// present (write or refresh actions) in the given dir, derived from
// the action set. Used to write the post-upgrade manifest.
func shippedSkillNames(actions []scaffold.Action, dir string) []string {
	prefix := dir + string(filepath.Separator)
	var names []string
	for _, a := range actions {
		if a.Kind == scaffold.KindRemove {
			continue
		}
		// Only top-level files under the dir count — skip the dir
		// itself and any nested paths (none today, future-proof).
		if rel := a.Rel; len(rel) > len(prefix) && rel[:len(prefix)] == prefix {
			name := rel[len(prefix):]
			if !containsSep(name) {
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return names
}

func containsSep(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == filepath.Separator || s[i] == '/' {
			return true
		}
	}
	return false
}

// plan assembles the full upgrade action set. Reads the project's
// docops.yaml so context_types propagate into emitted schemas.
func plan(opts Options) ([]scaffold.Action, error) {
	cfg := config.Default()
	if loaded, err := config.Load(filepath.Join(opts.Root, config.DefaultFilename)); err == nil {
		cfg = loaded
	}

	var actions []scaffold.Action

	// 0. Legacy cleanup — previous docops versions wrote Claude Code
	// slash commands into .claude/skills/docops/, but Claude Code reads
	// them from .claude/commands/. Remove every file still sitting in
	// the old location so users do not end up with duplicate skill
	// files after the bundle migrates to the new folder.
	for _, legacyDir := range legacyDocopsSkillDirs() {
		abs := filepath.Join(opts.Root, legacyDir)
		entries, err := os.ReadDir(abs)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read legacy skill dir %s: %w", legacyDir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			rel := filepath.Join(legacyDir, entry.Name())
			actions = append(actions, scaffold.Action{
				Path:   filepath.Join(opts.Root, rel),
				Rel:    rel,
				Kind:   scaffold.KindRemove,
				Reason: "legacy location — moved to .claude/commands/docops/",
			})
		}
	}

	// 1. JSON Schemas — always refreshed (docops-owned).
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
		actions = append(actions, scaffold.FileAction(opts.Root, rel, schemas[name], 0o644, true))
	}

	// 2. AGENTS.md and CLAUDE.md block refresh. Both files share the
	// same docops block (ADR-0024). For each: if the file exists, the
	// block is merged in place; if absent, the full template is
	// written so v0.1.x users gain CLAUDE.md on first upgrade after
	// this lands.
	agentsTmpl, err := templates.AgentsBlock()
	if err != nil {
		return nil, fmt.Errorf("read agents template: %w", err)
	}
	agentsAction, err := planMarkdownBlock(opts.Root, "AGENTS.md", agentsTmpl)
	if err != nil {
		return nil, err
	}
	actions = append(actions, agentsAction)

	claudeTmpl, err := templates.ClaudeBlock()
	if err != nil {
		return nil, fmt.Errorf("read claude template: %w", err)
	}
	claudeAction, err := planMarkdownBlock(opts.Root, "CLAUDE.md", claudeTmpl)
	if err != nil {
		return nil, err
	}
	actions = append(actions, claudeAction)

	// 3. Skills — sync each docops-owned dir against the shipped bundle.
	skills, err := scaffold.LoadShippedSkills()
	if err != nil {
		return nil, fmt.Errorf("read skills: %w", err)
	}
	skillNames := make([]string, 0, len(skills))
	for name := range skills {
		skillNames = append(skillNames, name)
	}
	sort.Strings(skillNames)

	for _, dir := range docopsSkillDirs() {
		dirActions, err := planSkillDir(opts, dir, skills, skillNames)
		if err != nil {
			return nil, err
		}
		actions = append(actions, dirActions...)
	}

	// 4. Opt-in: docops.yaml.
	if opts.Config {
		yamlBody, err := templates.DocopsYAML()
		if err != nil {
			return nil, fmt.Errorf("read docops.yaml template: %w", err)
		}
		actions = append(actions, scaffold.FileAction(opts.Root, "docops.yaml", yamlBody, 0o644, true))
	}

	// 5. Opt-in: pre-commit hook.
	if opts.Hook {
		hookBody, err := templates.PreCommitHook()
		if err != nil {
			return nil, fmt.Errorf("read pre-commit template: %w", err)
		}
		hookAction, err := planHook(opts, hookBody)
		if err != nil {
			return nil, err
		}
		actions = append(actions, hookAction)
	}

	return actions, nil
}

// planMarkdownBlock returns a refresh-or-create action for a
// docops-managed markdown file (AGENTS.md, CLAUDE.md). When the file
// is absent, it writes the template verbatim — upgrade now creates
// missing managed files so v0.1.x users gain CLAUDE.md on first
// post-ADR-0024 upgrade. When present, the docops block is merged in
// place; the rest of the file is preserved.
func planMarkdownBlock(rootAbs, rel string, tmpl []byte) (scaffold.Action, error) {
	abs := filepath.Join(rootAbs, rel)
	existing, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return scaffold.Action{
				Path:   abs,
				Rel:    rel,
				Kind:   scaffold.KindWriteFile,
				Body:   tmpl,
				Mode:   0o644,
				Reason: "create",
			}, nil
		}
		return scaffold.Action{}, err
	}
	merged, changed, reason := scaffold.MergeAgentsBlock(existing, tmpl)
	if !changed {
		return scaffold.Action{Path: abs, Rel: rel, Kind: scaffold.KindSkip, Reason: reason}, nil
	}
	return scaffold.Action{
		Path:   abs,
		Rel:    rel,
		Kind:   scaffold.KindMergeAgents,
		Body:   merged,
		Mode:   0o644,
		Reason: reason,
	}, nil
}

// planHook installs/refreshes .git/hooks/pre-commit. Only invoked
// when --hook is passed; otherwise upgrade leaves the user's hook
// chain alone.
func planHook(opts Options, body []byte) (scaffold.Action, error) {
	gitDir := filepath.Join(opts.Root, ".git")
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		return scaffold.Action{
			Path:   filepath.Join(gitDir, "hooks", "pre-commit"),
			Rel:    ".git/hooks/pre-commit",
			Kind:   scaffold.KindSkip,
			Reason: "no .git directory — install the hook manually from templates/hooks/pre-commit",
		}, nil
	}
	rel := ".git/hooks/pre-commit"
	abs := filepath.Join(opts.Root, rel)
	a := scaffold.Action{Path: abs, Rel: rel, Kind: scaffold.KindWriteFile, Body: body, Mode: 0o755}
	existing, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			a.Reason = "install"
			return a, nil
		}
		return scaffold.Action{}, err
	}
	if bytes.Equal(existing, body) {
		a.Kind = scaffold.KindSkip
		a.Reason = "already up to date"
		return a, nil
	}
	a.Reason = "overwrite drifted hook (--hook)"
	return a, nil
}

// planSkillDir walks one docops-owned skill directory and emits the
// create / refresh / remove / skip actions to bring it in line with
// the shipped bundle. The safety belt (manifest check) fires here:
// any present file that the manifest does not vouch for and the
// shipped bundle does not contain triggers ErrUnknownFiles.
func planSkillDir(opts Options, dir string, shipped map[string][]byte, shippedNames []string) ([]scaffold.Action, error) {
	absDir := filepath.Join(opts.Root, dir)

	present, dirExists, err := listSkillFiles(absDir)
	if err != nil {
		return nil, err
	}

	manifest, manifestExists, err := readManifest(absDir)
	if err != nil {
		return nil, err
	}

	// Safety belt: only enforce when a manifest exists. First-time
	// upgraders (v0.1.x users) get an implicit pass — anything in the
	// dir is treated as docops-owned and reconciled against the
	// shipped bundle.
	if manifestExists {
		shippedSet := stringSet(shippedNames)
		manifestSet := stringSet(manifest)
		var unknown []string
		for _, name := range present {
			if !shippedSet[name] && !manifestSet[name] {
				unknown = append(unknown, name)
			}
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			return nil, &ErrUnknownFiles{Dir: dir, Files: unknown}
		}
	}

	var actions []scaffold.Action

	// Ensure the directory exists (mkdir or skip).
	actions = append(actions, scaffold.DirAction(opts.Root, dir))

	// Refresh / create every shipped file.
	for _, name := range shippedNames {
		rel := filepath.Join(dir, name)
		actions = append(actions, scaffold.FileAction(opts.Root, rel, shipped[name], 0o644, true))
	}

	// Remove anything that is in the dir but not in the shipped bundle.
	// On first run (no manifest), this includes user files inside
	// docops/ — by design (see ADR-0021: "DocOps owns the docops/
	// subdirectory"). On subsequent runs, the safety belt above
	// already vetted the dir, so removals are scoped to manifest ∩
	// (not shipped) — i.e. genuine shipped removals.
	if dirExists {
		shippedSet := stringSet(shippedNames)
		for _, name := range present {
			if shippedSet[name] {
				continue
			}
			rel := filepath.Join(dir, name)
			actions = append(actions, scaffold.Action{
				Path:   filepath.Join(opts.Root, rel),
				Rel:    rel,
				Kind:   scaffold.KindRemove,
				Reason: "no longer in shipped bundle",
				Mode:   0o644,
			})
		}
	}

	// Also drop the manifest sentinel from "present" before any output
	// — it is internal metadata, not a skill file. listSkillFiles
	// already excludes dotfiles; nothing to do here.

	return actions, nil
}

// listSkillFiles returns the basenames of regular files directly
// inside dir (no recursion, no dotfiles). dirExists distinguishes a
// missing directory from an empty one so callers can choose to mkdir
// vs scan.
func listSkillFiles(dir string) (names []string, dirExists bool, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		name := e.Name()
		if len(name) > 0 && name[0] == '.' {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, true, nil
}

func stringSet(xs []string) map[string]bool {
	out := make(map[string]bool, len(xs))
	for _, x := range xs {
		out[x] = true
	}
	return out
}

// printPlan renders the action set with upgrade-specific sigils:
// `+` new, `~` refreshed, `-` removed, `=` up to date. Mirrors the
// shape promised by ADR-0021's "Output" section.
func printPlan(w io.Writer, actions []scaffold.Action, dry bool) {
	var changed, skipped int
	for _, a := range actions {
		if a.Kind == scaffold.KindSkip {
			skipped++
		} else {
			changed++
		}
	}
	verb := "applied"
	if dry {
		verb = "would apply"
	}
	fmt.Fprintf(w, "docops upgrade: %s %d change(s), skipped %d\n", verb, changed, skipped)
	for _, a := range actions {
		sigil, label := upgradeSigil(a)
		fmt.Fprintf(w, "  %s %-40s %s\n", sigil, a.Rel, label)
	}
}

// upgradeSigil maps a scaffold action to the sigil + parenthetical
// label used in upgrade's output. Init has its own (different)
// rendering; the two diverged because upgrade's output is a diff,
// init's is a creation log.
func upgradeSigil(a scaffold.Action) (string, string) {
	switch a.Kind {
	case scaffold.KindMkdir:
		return "+", "(new dir)"
	case scaffold.KindWriteFile:
		// FileAction sets Reason="create" for new files, else
		// "overwrite drifted content (--force)" or similar.
		if a.Reason == "create" || a.Reason == "install" {
			return "+", "(new)"
		}
		return "~", "(refreshed)"
	case scaffold.KindMergeAgents:
		return "~", "(block refreshed)"
	case scaffold.KindRemove:
		return "-", "(removed)"
	case scaffold.KindSkip:
		return "=", "(up to date)"
	}
	return "?", "(unknown)"
}
