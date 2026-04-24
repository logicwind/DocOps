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
	"strings"

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
	// Harnesses, if non-nil, restricts the upgrade to exactly this slug
	// set. nil means "use every registered harness" (the library default,
	// which tests rely on). The CLI does harness detection and passes an
	// explicit non-nil slice; library callers can do the same or leave
	// the field nil to write to every registered target. An empty
	// non-nil slice means "no harnesses" (skip all slash-command writes).
	Harnesses []string
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
	// docops-owned harness dir so subsequent upgrades scope deletions
	// correctly. Failures here are logged but do not fail the upgrade
	// — manifest is best-effort metadata.
	for _, h := range resolveHarnesses(opts.Harnesses) {
		mdir := h.ManifestDir()
		var names []string
		if h.Layout() == LayoutNestedSkillDir {
			// Codex manifest lists directory names (e.g. "docops-get"),
			// not individual file paths — cleanup is per-subdirectory.
			names = shippedSkillDirNames(actions, mdir)
		} else {
			names = shippedSkillNames(actions, mdir)
		}
		if err := writeManifest(filepath.Join(opts.Root, mdir), names); err != nil {
			fmt.Fprintf(opts.Out, "  warning: refresh manifest in %s: %v\n", mdir, err)
		}
	}
	printPlan(opts.Out, actions, false)
	return &Result{Actions: actions}, nil
}

// legacyDocopsSkillDirs lists paths that earlier docops releases wrote
// skill files into and that upgrade should actively clean up. Claude
// Code reads slash commands from .claude/commands/, not .claude/skills/,
// so v0.2.x wrote to the wrong folder; v0.3.x moves them.
func legacyDocopsSkillDirs() []string {
	return []string{".claude/skills/docops"}
}

// shippedSkillNames returns the basenames of files currently present
// (write or refresh actions) in the given manifestDir, derived from
// the action set. Used to write the post-upgrade manifest.
//
// For LayoutNestedFile harnesses, manifestDir is LocalDir()+"/docops";
// for LayoutFlatPrefixFile it is LocalDir() itself. Only the
// immediately-nested filenames (no path separators) are returned so
// the manifest stays a flat list.
func shippedSkillNames(actions []scaffold.Action, manifestDir string) []string {
	prefix := manifestDir + string(filepath.Separator)
	var names []string
	for _, a := range actions {
		if a.Kind == scaffold.KindRemove {
			continue
		}
		// Only top-level files under the manifestDir count — skip the
		// dir itself and any deeper nested paths.
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

	// 3. Commands — sync each docops-owned harness dir against the
	// shipped bundle, applying per-harness layout and frontmatter
	// transforms.
	shippedSkills, err := scaffold.LoadShippedSkills()
	if err != nil {
		return nil, fmt.Errorf("read skills: %w", err)
	}
	shippedCmds := make([]string, 0, len(shippedSkills))
	for name := range shippedSkills {
		// Strip the .md extension — shipped keys are basenames like "get.md".
		cmd := strings.TrimSuffix(name, ".md")
		shippedCmds = append(shippedCmds, cmd)
	}
	sort.Strings(shippedCmds)

	for _, h := range resolveHarnesses(opts.Harnesses) {
		hActions, err := planHarness(opts, h, shippedSkills, shippedCmds)
		if err != nil {
			return nil, err
		}
		actions = append(actions, hActions...)
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

// planHarness is the layout-aware replacement for planSkillDir. It
// emits create / refresh / remove / skip actions for one harness target
// by dispatching on h.Layout():
//
//   - LayoutNestedFile: owns h.ManifestDir() (= LocalDir()+"/docops").
//     Scans that directory for existing files.
//   - LayoutFlatPrefixFile: owns a flat subset of h.LocalDir() — only
//     files matching "docops-*.md" and the manifest. Other files in
//     LocalDir() belong to other tools and are never touched.
//   - LayoutNestedSkillDir: not yet implemented (Phase 2b). Returns an
//     explicit error so the missing branch is obvious.
//
// For each shipped command, the output path is:
//
//	filepath.Join(h.LocalDir(), h.FilenameFor(cmd))
//
// The file body is the shipped canonical bytes with
// h.TransformFrontmatter applied to the YAML frontmatter block.
func planHarness(opts Options, h Harness, shipped map[string][]byte, shippedCmds []string) ([]scaffold.Action, error) {
	switch h.Layout() {
	case LayoutNestedFile:
		return planNestedFileHarness(opts, h, shipped, shippedCmds)
	case LayoutFlatPrefixFile:
		return planFlatPrefixHarness(opts, h, shipped, shippedCmds)
	case LayoutNestedSkillDir:
		return planNestedSkillDirHarness(opts, h, shipped, shippedCmds)
	default:
		return nil, fmt.Errorf("planHarness: unknown layout %d for harness %q", h.Layout(), h.Slug())
	}
}

// planNestedFileHarness handles LayoutNestedFile harnesses (Claude, Cursor).
// The owned directory is h.ManifestDir() (= h.LocalDir()+"/docops").
// Files land at filepath.Join(h.LocalDir(), h.FilenameFor(cmd)).
func planNestedFileHarness(opts Options, h Harness, shipped map[string][]byte, shippedCmds []string) ([]scaffold.Action, error) {
	// The manifest lives in ManifestDir, which is also the fully-owned
	// subdirectory for LayoutNestedFile harnesses.
	manifestDir := h.ManifestDir()
	absManifestDir := filepath.Join(opts.Root, manifestDir)

	present, dirExists, err := listSkillFiles(absManifestDir)
	if err != nil {
		return nil, err
	}

	manifest, manifestExists, err := readManifest(absManifestDir)
	if err != nil {
		return nil, err
	}

	// Build the set of shipped basenames (e.g. "get.md") for safety belt
	// and removal logic — these are the filenames inside manifestDir.
	shippedBasenames := make([]string, len(shippedCmds))
	for i, cmd := range shippedCmds {
		shippedBasenames[i] = cmd + ".md"
	}

	if manifestExists {
		shippedSet := stringSet(shippedBasenames)
		manifestSet := stringSet(manifest)
		var unknown []string
		for _, name := range present {
			if !shippedSet[name] && !manifestSet[name] {
				unknown = append(unknown, name)
			}
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			return nil, &ErrUnknownFiles{Dir: manifestDir, Files: unknown}
		}
	}

	var actions []scaffold.Action

	// Ensure the manifest directory exists (mkdir or skip).
	// For NestedFile this is LocalDir()/docops — create if absent.
	actions = append(actions, scaffold.DirAction(opts.Root, manifestDir))

	// Refresh / create every shipped command.
	for _, cmd := range shippedCmds {
		srcBody := shipped[cmd+".md"]
		outBody, err := applyTransform(srcBody, h.TransformFrontmatter)
		if err != nil {
			return nil, fmt.Errorf("transform frontmatter for %s/%s: %w", h.Slug(), cmd, err)
		}
		rel := filepath.Join(h.LocalDir(), h.FilenameFor(cmd))
		actions = append(actions, scaffold.FileAction(opts.Root, rel, outBody, 0o644, true))
	}

	// Remove files in the owned dir that are no longer in the shipped bundle.
	if dirExists {
		shippedSet := stringSet(shippedBasenames)
		for _, name := range present {
			if shippedSet[name] {
				continue
			}
			rel := filepath.Join(manifestDir, name)
			actions = append(actions, scaffold.Action{
				Path:   filepath.Join(opts.Root, rel),
				Rel:    rel,
				Kind:   scaffold.KindRemove,
				Reason: "no longer in shipped bundle",
				Mode:   0o644,
			})
		}
	}

	return actions, nil
}

// planFlatPrefixHarness handles LayoutFlatPrefixFile harnesses (OpenCode).
// The owned file set is the subset of h.LocalDir() matching "docops-*.md"
// plus the manifest sentinel. Other files in LocalDir() are never touched.
func planFlatPrefixHarness(opts Options, h Harness, shipped map[string][]byte, shippedCmds []string) ([]scaffold.Action, error) {
	localDir := h.LocalDir()       // e.g. ".opencode/command"
	manifestDir := h.ManifestDir() // same as localDir for FlatPrefix
	absLocalDir := filepath.Join(opts.Root, localDir)

	// List only docops-owned files (docops-*.md) in the flat dir.
	present, dirExists, err := listFlatPrefixFiles(absLocalDir)
	if err != nil {
		return nil, err
	}

	manifest, manifestExists, err := readManifest(filepath.Join(opts.Root, manifestDir))
	if err != nil {
		return nil, err
	}

	// Build the set of shipped flat filenames (e.g. "docops-get.md").
	shippedFlat := make([]string, len(shippedCmds))
	for i, cmd := range shippedCmds {
		shippedFlat[i] = h.FilenameFor(cmd) // "docops-get.md"
	}

	if manifestExists {
		shippedSet := stringSet(shippedFlat)
		manifestSet := stringSet(manifest)
		var unknown []string
		for _, name := range present {
			if !shippedSet[name] && !manifestSet[name] {
				unknown = append(unknown, name)
			}
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			return nil, &ErrUnknownFiles{Dir: localDir, Files: unknown}
		}
	}

	var actions []scaffold.Action

	// Ensure the target directory exists.
	actions = append(actions, scaffold.DirAction(opts.Root, localDir))

	// Refresh / create every shipped command with frontmatter transform.
	for _, cmd := range shippedCmds {
		srcBody := shipped[cmd+".md"]
		outBody, err := applyTransform(srcBody, h.TransformFrontmatter)
		if err != nil {
			return nil, fmt.Errorf("transform frontmatter for %s/%s: %w", h.Slug(), cmd, err)
		}
		rel := filepath.Join(h.LocalDir(), h.FilenameFor(cmd))
		actions = append(actions, scaffold.FileAction(opts.Root, rel, outBody, 0o644, true))
	}

	// Remove owned files that are no longer in the shipped bundle.
	if dirExists {
		shippedSet := stringSet(shippedFlat)
		for _, name := range present {
			if shippedSet[name] {
				continue
			}
			rel := filepath.Join(localDir, name)
			actions = append(actions, scaffold.Action{
				Path:   filepath.Join(opts.Root, rel),
				Rel:    rel,
				Kind:   scaffold.KindRemove,
				Reason: "no longer in shipped bundle",
				Mode:   0o644,
			})
		}
	}

	return actions, nil
}

// planNestedSkillDirHarness handles LayoutNestedSkillDir harnesses (Codex).
// Each command lands at filepath.Join(h.LocalDir(), "docops-<cmd>", "SKILL.md").
// The manifest (at h.ManifestDir()/.docops-manifest) lists directory names
// (e.g. "docops-get"), not file paths, so cleanup is per-subdirectory.
//
// NOTE on name: injection:
// Codex skills require a name: field equal to the skill directory name
// (e.g. "docops-get"). TransformFrontmatter is purposely pure and has no
// knowledge of the command name, so after calling it we inject
// name: "docops-<cmd>" into the resulting map before serialization.
// This keeps the Harness interface stable. See also the comment on codexAdapter.
func planNestedSkillDirHarness(opts Options, h Harness, shipped map[string][]byte, shippedCmds []string) ([]scaffold.Action, error) {
	localDir := h.LocalDir()       // e.g. ".codex/skills"
	manifestDir := h.ManifestDir() // same as localDir for NestedSkillDir
	absLocalDir := filepath.Join(opts.Root, localDir)

	// List present docops-* subdirectories (by name, not file path).
	presentDirs, dirExists, err := listNestedSkillDirs(absLocalDir)
	if err != nil {
		return nil, err
	}

	manifest, manifestExists, err := readManifest(filepath.Join(opts.Root, manifestDir))
	if err != nil {
		return nil, err
	}

	// Build the set of shipped directory names (e.g. "docops-get").
	shippedDirs := make([]string, len(shippedCmds))
	for i, cmd := range shippedCmds {
		shippedDirs[i] = "docops-" + cmd
	}

	// Safety belt: if we have a manifest, reject any docops-* dirs not
	// accounted for by either the shipped set or the previous manifest.
	if manifestExists {
		shippedSet := stringSet(shippedDirs)
		manifestSet := stringSet(manifest)
		var unknown []string
		for _, name := range presentDirs {
			if !shippedSet[name] && !manifestSet[name] {
				unknown = append(unknown, name)
			}
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			return nil, &ErrUnknownFiles{Dir: localDir, Files: unknown}
		}
	}

	var actions []scaffold.Action

	// Ensure the parent skills directory exists.
	actions = append(actions, scaffold.DirAction(opts.Root, localDir))

	// Refresh / create every shipped command with frontmatter transform.
	// Per-command: inject name: "docops-<cmd>" after the pure transform.
	for _, cmd := range shippedCmds {
		skillDirName := "docops-" + cmd
		skillDirRel := filepath.Join(localDir, skillDirName)

		// Ensure the per-command subdirectory exists.
		actions = append(actions, scaffold.DirAction(opts.Root, skillDirRel))

		srcBody := shipped[cmd+".md"]

		// Apply the harness's pure transform, then inject name:.
		fm, node, body, err := parseFrontmatter(srcBody)
		if err != nil {
			return nil, fmt.Errorf("parse frontmatter for %s/%s: %w", h.Slug(), cmd, err)
		}
		outFM, err := h.TransformFrontmatter(fm)
		if err != nil {
			return nil, fmt.Errorf("transform frontmatter for %s/%s: %w", h.Slug(), cmd, err)
		}
		// Inject name: "docops-<cmd>" — the per-command bit that
		// TransformFrontmatter cannot set because it is a pure function
		// with no knowledge of the command name.
		outFM["name"] = skillDirName
		outBody, err := serializeFrontmatter(outFM, node, body)
		if err != nil {
			return nil, fmt.Errorf("serialize frontmatter for %s/%s: %w", h.Slug(), cmd, err)
		}

		rel := filepath.Join(h.LocalDir(), h.FilenameFor(cmd)) // e.g. .codex/skills/docops-get/SKILL.md
		actions = append(actions, scaffold.FileAction(opts.Root, rel, outBody, 0o644, true))
	}

	// Remove docops-* subdirs that are no longer in the shipped bundle.
	// Minimum acceptable: emit KindRemove for docops-<stale>/SKILL.md.
	// An empty dir may remain behind; that is acceptable for v0.
	if dirExists {
		shippedSet := stringSet(shippedDirs)
		for _, name := range presentDirs {
			if shippedSet[name] {
				continue
			}
			skillFile := filepath.Join(localDir, name, "SKILL.md")
			actions = append(actions, scaffold.Action{
				Path:   filepath.Join(opts.Root, skillFile),
				Rel:    skillFile,
				Kind:   scaffold.KindRemove,
				Reason: "no longer in shipped bundle",
				Mode:   0o644,
			})
		}
	}

	return actions, nil
}

// listNestedSkillDirs returns the names of subdirectories in dir whose
// name starts with "docops-" and that contain a SKILL.md file. dirExists
// distinguishes a missing directory from an empty one.
func listNestedSkillDirs(dir string) (names []string, dirExists bool, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "docops-") {
			continue
		}
		// Only count dirs that contain a SKILL.md (to match GSD's listCodexSkillNames).
		skillPath := filepath.Join(dir, name, "SKILL.md")
		if _, err := os.Stat(skillPath); err == nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, true, nil
}

// shippedSkillDirNames returns the directory names of actions that
// represent a SKILL.md write under manifestDir. Used to build the
// Codex manifest, which lists directory names (e.g. "docops-get") not
// file paths. Only immediate child directory names are returned.
func shippedSkillDirNames(actions []scaffold.Action, manifestDir string) []string {
	prefix := manifestDir + string(filepath.Separator)
	var names []string
	for _, a := range actions {
		if a.Kind == scaffold.KindRemove {
			continue
		}
		rel := a.Rel
		if len(rel) <= len(prefix) || rel[:len(prefix)] != prefix {
			continue
		}
		// rel is like ".codex/skills/docops-get/SKILL.md"
		// after stripping prefix: "docops-get/SKILL.md"
		rest := rel[len(prefix):]
		// We want only the first path segment (the skill dir name).
		sep := strings.IndexByte(rest, filepath.Separator)
		if sep < 0 {
			// This is a flat file, not a skill dir — skip.
			continue
		}
		dirName := rest[:sep]
		if strings.HasPrefix(dirName, "docops-") {
			names = append(names, dirName)
		}
	}
	sort.Strings(names)
	// Deduplicate (DirAction + FileAction both appear under the same skill dir).
	var deduped []string
	for i, n := range names {
		if i == 0 || n != names[i-1] {
			deduped = append(deduped, n)
		}
	}
	return deduped
}

// listFlatPrefixFiles returns the basenames of regular files in dir
// whose name matches "docops-*.md" (the docops-owned subset of a flat
// command directory). Dotfiles and files not matching the prefix are
// skipped — they belong to other tools.
func listFlatPrefixFiles(dir string) (names []string, dirExists bool, err error) {
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
		if strings.HasPrefix(name, "docops-") && strings.HasSuffix(name, ".md") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, true, nil
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
