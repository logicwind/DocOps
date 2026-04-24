package upgrader

import (
	"os"
	"path/filepath"
)

// codexAdapter delivers /docops:* commands to Codex as nested skill directories.
//
// LocalDir:  .codex/skills   (parent; no docops/ subdirectory)
// FilenameFor("get") = "docops-get/SKILL.md"
// ManifestDir: .codex/skills  (manifest sits in the parent, lists directory names)
// Layout: LayoutNestedSkillDir — each command becomes a directory docops-<cmd>/
// containing SKILL.md, so `/docops:get` maps to .codex/skills/docops-get/SKILL.md.
//
// GlobalDir precedence (mirrors GSD lines 261–269):
//  1. $CODEX_HOME/skills  (docops does not support --config-dir; omitted)
//  2. ~/.codex/skills
//
// Manifest entries are directory names (e.g. "docops-get"), not file paths.
// Cleanup removes the whole docops-<cmd>/ subdirectory on de-ship.
//
// NOTE on name: injection (Phase 2b design):
//
// TransformFrontmatter is a pure function with no knowledge of the command name.
// However, Codex skills require a name: field set to the skill directory name
// (e.g. "docops-get"). This is injected by planNestedSkillDirHarness right
// before serialization — after TransformFrontmatter runs — so TransformFrontmatter
// itself stays pure and the interface stays stable. Future harnesses that need
// per-command name injection should follow the same closure pattern in their
// writer branch rather than extending the Harness interface.
type codexAdapter struct{}

func (codexAdapter) Slug() string     { return "codex" }
func (codexAdapter) LocalDir() string { return ".codex/skills" }

func (codexAdapter) GlobalDir() (string, bool) {
	// 1. $CODEX_HOME/skills
	if v := os.Getenv("CODEX_HOME"); v != "" {
		return filepath.Join(v, "skills"), true
	}
	// 2. ~/.codex/skills
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	return filepath.Join(home, ".codex", "skills"), true
}

func (codexAdapter) Layout() Layout { return LayoutNestedSkillDir }

// FilenameFor returns the path of the SKILL.md file relative to LocalDir().
// For example, FilenameFor("get") = "docops-get/SKILL.md".
func (codexAdapter) FilenameFor(cmd string) string {
	return filepath.Join("docops-"+cmd, "SKILL.md")
}

// ManifestDir returns LocalDir — the manifest sits in .codex/skills/.docops-manifest
// and lists directory names (not file paths) so cleanup is per-subdirectory.
func (codexAdapter) ManifestDir() string { return ".codex/skills" }

// TransformFrontmatter converts Claude-canonical frontmatter into the Codex
// skill dialect (per GSD convertClaudeCommandToCodexSkill):
//
//   - Drop "allowed-tools:" — Codex skills do not use this field.
//   - Preserve "description:" verbatim.
//   - Drop "name:" — the caller (planNestedSkillDirHarness) injects the correct
//     name: "docops-<cmd>" after this transform runs. See the NOTE comment on
//     codexAdapter for rationale.
//   - Drop all other Claude-only keys (e.g. "argument-hint" is preserved if
//     Codex could use it, but currently dropped along with unrecognised fields).
//
// Only "description" survives verbatim; "name:" is injected by the writer.
func (codexAdapter) TransformFrontmatter(src map[string]any) (map[string]any, error) {
	out := make(map[string]any, 2)
	// Preserve description if present.
	if desc, ok := src["description"]; ok {
		out["description"] = desc
	}
	// NOTE: "name:" is intentionally NOT set here.
	// planNestedSkillDirHarness injects name: "docops-<cmd>" per-command
	// after calling TransformFrontmatter, so this function stays pure.
	// Dropped fields: allowed-tools, argument-hint, and any other Claude-only keys.
	return out, nil
}
