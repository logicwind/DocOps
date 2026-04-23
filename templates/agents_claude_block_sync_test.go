package templates

import (
	"strings"
	"testing"
)

// extractDocopsBlock returns the content between the docops markers
// in the given template body. Inlined here to avoid importing
// internal/scaffold (which would create a templates → scaffold edge
// that doesn't exist anywhere else; templates is the leafmost package).
func extractDocopsBlock(body []byte) string {
	const start = "<!-- docops:start -->"
	const end = "<!-- docops:end -->"
	s := string(body)
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	i += len(start)
	j := strings.Index(s, end)
	if j < 0 || j < i {
		return ""
	}
	return s[i:j]
}

// TestAgentsClaudeBlocksInSync enforces ADR-0024: the docops block in
// AGENTS.md.tmpl and CLAUDE.md.tmpl must be byte-identical so users
// see the same invariants regardless of which file their tool reads.
// Drift in either template fails this test at build time.
func TestAgentsClaudeBlocksInSync(t *testing.T) {
	agents, err := AgentsBlock()
	if err != nil {
		t.Fatalf("AgentsBlock: %v", err)
	}
	claude, err := ClaudeBlock()
	if err != nil {
		t.Fatalf("ClaudeBlock: %v", err)
	}

	a := extractDocopsBlock(agents)
	c := extractDocopsBlock(claude)
	if a == "" {
		t.Fatal("AGENTS.md.tmpl is missing the docops block markers")
	}
	if c == "" {
		t.Fatal("CLAUDE.md.tmpl is missing the docops block markers")
	}
	if a != c {
		t.Errorf("docops block in AGENTS.md.tmpl and CLAUDE.md.tmpl have drifted.\n--- AGENTS ---\n%s\n--- CLAUDE ---\n%s", a, c)
	}
}
