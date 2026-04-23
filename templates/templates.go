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

// Skills returns a map of skill leaf filename (e.g. "init.md") to file body.
// Callers write these into `.claude/commands/docops/` and
// `.cursor/commands/docops/` — both are the slash-command directories
// for their respective agent tools.
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
		body, err := tree.ReadFile("skills/docops/" + e.Name())
		if err != nil {
			return nil, err
		}
		out[e.Name()] = body
	}
	return out, nil
}
