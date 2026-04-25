// Package templates bundles the product-layer assets that `docops init`
// writes into user repositories: AGENTS.md block, docops.yaml, pre-commit
// hook, and the `/docops:*` skill bundle.
//
// Per ADR-0016 these files are the canonical source — the Go package is
// a thin accessor that embeds them via go:embed so init does not depend
// on the source repo being present at runtime.
package templates

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed AGENTS.md.tmpl CLAUDE.md.tmpl docops.yaml.tmpl hooks/pre-commit skills/docops
var tree embed.FS

// AgentsBlock returns the body of templates/AGENTS.md.tmpl.
func AgentsBlock() ([]byte, error) {
	return tree.ReadFile("AGENTS.md.tmpl")
}

// ClaudeBlock returns the body of templates/CLAUDE.md.tmpl. The
// docops block inside (between `<!-- docops:start -->` and
// `<!-- docops:end -->`) is byte-identical to AgentsBlock; only the
// preamble and footer differ. See ADR-0024.
func ClaudeBlock() ([]byte, error) {
	return tree.ReadFile("CLAUDE.md.tmpl")
}

// DocopsYAML returns the body of templates/docops.yaml.tmpl.
func DocopsYAML() ([]byte, error) {
	return tree.ReadFile("docops.yaml.tmpl")
}

// PreCommitHook returns the body of templates/hooks/pre-commit.
func PreCommitHook() ([]byte, error) {
	return tree.ReadFile("hooks/pre-commit")
}

// Skills returns a map of per-command leaf filename (e.g. "init.md") to
// file body. Callers write these into the appropriate per-harness target
// directory (`.claude/commands/docops/`, `.cursor/commands/docops/`,
// `.opencode/command/`, or — for skill-bundle harnesses — alongside
// SKILL.md inside the bundle directory).
//
// SKILL.md is intentionally excluded from this map: it is the
// skill-bundle entry point, not a /docops:* command. Use
// SkillBundleEntry() to fetch it.
func Skills() (map[string][]byte, error) {
	out := make(map[string][]byte)
	entries, err := fs.ReadDir(tree, "skills/docops")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if e.Name() == "SKILL.md" {
			continue
		}
		body, err := tree.ReadFile("skills/docops/" + e.Name())
		if err != nil {
			return nil, err
		}
		out[e.Name()] = body
	}
	return out, nil
}

// SkillBundleEntry returns the body of templates/skills/docops/SKILL.md —
// the entry-point file for skill-bundle harnesses (Codex). It is shipped
// verbatim into the bundle directory and is the file Codex auto-loads when
// it decides the docops skill is relevant.
func SkillBundleEntry() ([]byte, error) {
	return tree.ReadFile("skills/docops/SKILL.md")
}
