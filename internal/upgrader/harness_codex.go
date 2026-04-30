package upgrader

import (
	"os"
	"path/filepath"
)

// codexAdapter delivers /docops:* subroutines to Codex as a single skill
// bundle. The whole DocOps surface is one skill from Codex's point of view.
//
// LocalDir:  .codex/skills        (parent of the bundle)
// FilenameFor("get") = "docops/cookbook/get.md"
// ManifestDir: .codex/skills/docops   (the bundle itself)
// Layout: LayoutSkillBundle — one bundle directory containing
//
//	docops/
//	  SKILL.md            ← entry point, auto-loaded by description matching
//	  cookbook/
//	    audit.md          ← per-subroutine files referenced by SKILL.md
//	    close.md
//	    get.md
//	    ... etc
//
// Per ADR-0031, per-subroutine files live in a `cookbook/` subdirectory
// inside the bundle. SKILL.md is the umbrella router and points at
// `cookbook/<verb>.md` for each chapter.
//
// SKILL.md is shipped verbatim from templates/skills/docops/SKILL.md and
// is the only file Codex auto-loads; the per-subroutine files are pulled
// in by the agent on demand based on the index in SKILL.md.
//
// GlobalDir precedence:
//  1. $CODEX_HOME/skills
//  2. ~/.codex/skills
//
// TransformFrontmatter applies to the per-subroutine files (drops
// allowed-tools and name; preserves description). SKILL.md is authored
// in Codex format and is not transformed.
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

func (codexAdapter) Layout() Layout { return LayoutSkillBundle }

// FilenameFor returns the path of the per-subroutine file relative to
// LocalDir(). Per ADR-0031, per-subroutine files live under
// `cookbook/`: FilenameFor("get") = "docops/cookbook/get.md".
//
// SKILL.md is not a subroutine and is written separately by the
// LayoutSkillBundle planner.
func (codexAdapter) FilenameFor(cmd string) string {
	return filepath.Join("docops", "cookbook", cmd+".md")
}

// ManifestDir returns the bundle directory itself: the manifest sits at
// .codex/skills/docops/.docops-manifest and lists the basenames of every
// docops-owned file inside the bundle (SKILL.md plus each <cmd>.md).
func (codexAdapter) ManifestDir() string {
	return filepath.Join(".codex/skills", "docops")
}

// TransformFrontmatter rewrites Claude-canonical frontmatter for a
// per-subroutine file (e.g. get.md) into the Codex dialect:
//
//   - Drop "allowed-tools:" — Codex skills do not use this field.
//   - Drop "name:" — the bundle as a whole has a single name (set in
//     SKILL.md); per-subroutine files do not need their own.
//   - Preserve "description:" verbatim — useful when the agent reads a
//     subroutine file directly.
//
// SKILL.md itself is shipped pre-formatted and bypasses this transform.
func (codexAdapter) TransformFrontmatter(src map[string]any) (map[string]any, error) {
	out := make(map[string]any, 1)
	if desc, ok := src["description"]; ok {
		out["description"] = desc
	}
	return out, nil
}
