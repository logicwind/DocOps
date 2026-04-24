package upgrader

import (
	"fmt"
	"path/filepath"
)

// Layout describes how a harness maps command names to filenames in its
// target directory. The three forms cover all harnesses in ADR-0028:
//
//   - LayoutNestedFile — command lives at <prefix>/<cmd>.md, invoked as
//     /<prefix>:<cmd>. Used by Claude Code and Cursor.
//   - LayoutFlatPrefixFile — command lives at <prefix>-<cmd>.md, invoked
//     as /<prefix>-<cmd>. Used by OpenCode and Kilo.
//   - LayoutNestedSkillDir — command lives at <prefix>-<cmd>/SKILL.md,
//     i.e. each command is a directory. Used by Codex.
type Layout int

const (
	// LayoutNestedFile is the nested form: docops/get.md → /docops:get.
	// Claude Code and Cursor use this layout.
	LayoutNestedFile Layout = iota

	// LayoutFlatPrefixFile is the flat-prefix form: docops-get.md → /docops-get.
	// OpenCode and Kilo use this layout.
	LayoutFlatPrefixFile

	// LayoutNestedSkillDir is the nested-skill-dir form: docops-get/SKILL.md.
	// Codex uses this layout — each command becomes a directory containing SKILL.md.
	LayoutNestedSkillDir
)

// Harness describes one agent tool's slash-command conventions. A
// registry of Harness values drives docops upgrade so adding a new
// target (e.g. OpenCode, Codex) is a matter of registering a new
// adapter without changing the write loop.
//
// The canonical command source is templates/skills/docops/*.md in
// Claude format. Each harness may transform the frontmatter via
// TransformFrontmatter before the file is written into its target dir.
//
// # Path composition convention
//
// LocalDir() returns the parent directory the harness reads command files
// from — it never includes "docops/" or any other docops-specific suffix.
// FilenameFor(cmd) returns the path of a single command file relative to
// LocalDir(). The writer always composes the full path as:
//
//	filepath.Join(h.LocalDir(), h.FilenameFor(cmd))
//
// ManifestDir() returns the directory that holds the .docops-manifest
// sentinel. For LayoutNestedFile harnesses this is LocalDir()+"/docops";
// for LayoutFlatPrefixFile harnesses it is LocalDir() itself, because
// all docops-owned files sit flat in the same directory as the manifest.
//
// This convention was established in TP-033 Phase 2; do not deviate in
// future adapters (e.g. Codex Phase 2b) without updating this comment.
type Harness interface {
	// Slug is the short machine-readable name (e.g. "claude", "cursor").
	Slug() string

	// LocalDir is the project-local parent directory the harness reads
	// slash commands from (relative to the project root). It never
	// includes a "docops/" suffix — FilenameFor encodes the sub-path.
	// Example: ".claude/commands" (not ".claude/commands/docops").
	LocalDir() string

	// GlobalDir returns the user-level directory for the harness and
	// whether this harness has a global concept at all. The path is
	// XDG-aware where applicable. ok=false means the harness has no
	// global config dir concept (or it is not yet implemented).
	GlobalDir() (path string, ok bool)

	// Layout returns the filename scheme the harness uses. Callers must
	// dispatch on this to determine where each command file lands and
	// what directory structure to create.
	Layout() Layout

	// FilenameFor returns the path of a command file relative to
	// LocalDir(). For LayoutNestedFile it is "docops/<cmd>.md"; for
	// LayoutFlatPrefixFile it is "docops-<cmd>.md"; for
	// LayoutNestedSkillDir it is "docops-<cmd>/SKILL.md".
	//
	// Full on-disk path: filepath.Join(h.LocalDir(), h.FilenameFor(cmd)).
	FilenameFor(cmd string) string

	// ManifestDir returns the directory that holds the .docops-manifest
	// file (relative to project root). For LayoutNestedFile harnesses
	// (Claude, Cursor) this is filepath.Join(LocalDir(), "docops").
	// For LayoutFlatPrefixFile (OpenCode) it is LocalDir() itself.
	ManifestDir() string

	// TransformFrontmatter rewrites Claude-canonical frontmatter (the
	// format in templates/skills/docops/*.md) into the harness dialect.
	// The function is pure — it must not perform I/O. For harnesses that
	// accept Claude frontmatter verbatim (Phase 1: Claude, Cursor), this
	// is an identity function.
	TransformFrontmatter(src map[string]any) (map[string]any, error)
}

// claudeAdapter delivers /docops:* commands to Claude Code.
//
// LocalDir:  .claude/commands   (parent; never includes docops/)
// FilenameFor("get") = "docops/get.md"
// ManifestDir: .claude/commands/docops
// Layout: NestedFile — docops/get.md → /docops:get
type claudeAdapter struct{}

func (claudeAdapter) Slug() string     { return "claude" }
func (claudeAdapter) LocalDir() string { return ".claude/commands" }
func (claudeAdapter) GlobalDir() (string, bool) {
	// Claude Code's global dir is ~/.claude/commands.
	// Returning ok=false for now — Phase 3 will wire global-dir detection.
	return "", false
}
func (claudeAdapter) Layout() Layout { return LayoutNestedFile }
func (claudeAdapter) FilenameFor(cmd string) string {
	return filepath.Join("docops", cmd+".md")
}
func (claudeAdapter) ManifestDir() string {
	return filepath.Join(".claude/commands", "docops")
}
func (claudeAdapter) TransformFrontmatter(src map[string]any) (map[string]any, error) {
	// Claude is the canonical format; identity transform.
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out, nil
}

// cursorAdapter delivers /docops:* commands to Cursor.
//
// LocalDir:  .cursor/commands   (parent; never includes docops/)
// FilenameFor("get") = "docops/get.md"
// ManifestDir: .cursor/commands/docops
// Layout: NestedFile — docops/get.md → /docops:get
type cursorAdapter struct{}

func (cursorAdapter) Slug() string     { return "cursor" }
func (cursorAdapter) LocalDir() string { return ".cursor/commands" }
func (cursorAdapter) GlobalDir() (string, bool) {
	// Cursor has a global dir at ~/.cursor/commands but Phase 3
	// handles global-dir detection; return ok=false for now.
	return "", false
}
func (cursorAdapter) Layout() Layout { return LayoutNestedFile }
func (cursorAdapter) FilenameFor(cmd string) string {
	return filepath.Join("docops", cmd+".md")
}
func (cursorAdapter) ManifestDir() string {
	return filepath.Join(".cursor/commands", "docops")
}
func (cursorAdapter) TransformFrontmatter(src map[string]any) (map[string]any, error) {
	// Cursor currently accepts Claude-format frontmatter verbatim.
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out, nil
}

// registry is the ordered list of all registered harness adapters.
// docops upgrade iterates this list to write commands into each target.
// To add a new harness (Codex, …) register it here.
var registry = []Harness{
	claudeAdapter{},
	cursorAdapter{},
	openCodeAdapter{},
}

// registeredHarnesses returns the current adapter registry. Callers
// that need to iterate all targets should use this rather than
// accessing registry directly; it is a shallow copy so tests can swap
// the slice without races.
func registeredHarnesses() []Harness {
	out := make([]Harness, len(registry))
	copy(out, registry)
	return out
}

// harnessLocalDirs returns the LocalDir() of every registered harness.
// This replaces the deleted docopsSkillDirs() function.
func harnessLocalDirs() []string {
	harnesses := registeredHarnesses()
	dirs := make([]string, len(harnesses))
	for i, h := range harnesses {
		dirs[i] = h.LocalDir()
	}
	return dirs
}

// FilenameForLayout returns the filename for cmd under the given layout,
// following the conventions documented on Layout. This is a free function
// that complements Harness.FilenameFor — useful when constructing paths
// without a concrete adapter.
func FilenameForLayout(layout Layout, cmd string) (string, error) {
	switch layout {
	case LayoutNestedFile:
		return filepath.Join("docops", cmd+".md"), nil
	case LayoutFlatPrefixFile:
		return fmt.Sprintf("docops-%s.md", cmd), nil
	case LayoutNestedSkillDir:
		return filepath.Join(fmt.Sprintf("docops-%s", cmd), "SKILL.md"), nil
	default:
		return "", fmt.Errorf("unknown layout %d", layout)
	}
}
