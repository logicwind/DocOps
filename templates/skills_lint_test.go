package templates

// skills_lint_test.go — static lint over templates/skills/docops/*.md.
//
// Rules enforced:
//  1. Every `docops <sub>` invocation in a fenced code block must use a
//     known subcommand from the shipped CLI.
//  2. `docops new <kind>` — kind must be ctx | adr | task.
//  3. Every long flag (--foo) must appear in the per-subcommand allowlist
//     derived from cmd/docops/cmd_*.go.
//
// No external dependencies; no binary on PATH required.

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

// knownSubcmds is the shipped CLI surface for v0.1.1.
var knownSubcmds = map[string]bool{
	"init":         true,
	"validate":     true,
	"index":        true,
	"state":        true,
	"audit":        true,
	"new":          true,
	"schema":       true,
	"refresh":      true,
	"update-check": true,
}

// knownNewKinds are valid arguments for `docops new`.
var knownNewKinds = map[string]bool{
	"ctx":  true,
	"adr":  true,
	"task": true,
}

// flagAllowlist maps subcommand → set of valid long flags.
// Built from cmd/docops/cmd_*.go — update here whenever a flag is added.
var flagAllowlist = map[string]map[string]bool{
	"init": {
		"--dry-run":   true,
		"--force":     true,
		"--no-skills": true,
		"--yes":       true,
	},
	"validate": {
		"--json": true,
		"--only": true,
	},
	"index": {
		"--json": true,
	},
	"state": {},
	"audit": {
		"--json":               true,
		"--only":               true,
		"--include-not-needed": true,
	},
	"schema": {
		"--stdout": true,
	},
	"refresh": {
		"--json": true,
	},
	// new ctx / new adr / new task share the "new" key; we resolve
	// per-kind flags under "new/<kind>" and fall back to "new".
	"new": {
		// common to all new subcommands
		"--no-open": true,
		"--json":    true,
	},
	"new/ctx": {
		"--type":      true,
		"--no-open":   true,
		"--json":      true,
		"--body":      true,
		"--body-file": true,
	},
	"new/adr": {
		"--related":   true,
		"--no-open":   true,
		"--json":      true,
		"--body":      true,
		"--body-file": true,
	},
	"new/task": {
		"--requires":  true,
		"--priority":  true,
		"--assignee":  true,
		"--no-open":   true,
		"--json":      true,
		"--body":      true,
		"--body-file": true,
	},
}

func TestSkillsLint(t *testing.T) {
	skills, err := Skills()
	if err != nil {
		t.Fatalf("Skills(): %v", err)
	}
	if len(skills) == 0 {
		t.Fatal("Skills() returned empty map — embed may be broken")
	}

	for name, body := range skills {
		lintSkill(t, name, body)
	}
}

// lintSkill checks every `docops ...` line inside fenced code blocks in body.
func lintSkill(t *testing.T, filename string, body []byte) {
	t.Helper()
	inFence := false
	sc := bufio.NewScanner(bytes.NewReader(body))
	lineNum := 0
	for sc.Scan() {
		lineNum++
		line := sc.Text()
		trimmed := strings.TrimSpace(line)

		// Toggle fenced-code-block state.
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if !inFence {
			continue
		}

		// Only care about lines that look like a docops invocation.
		if !strings.HasPrefix(trimmed, "docops ") {
			continue
		}

		tokens := strings.Fields(trimmed)
		// tokens[0] == "docops"
		if len(tokens) < 2 {
			t.Errorf("%s:%d: bare `docops` with no subcommand", filename, lineNum)
			continue
		}
		sub := tokens[1]
		if !knownSubcmds[sub] {
			t.Errorf("%s:%d: unknown subcommand %q (known: init validate index state audit new schema)",
				filename, lineNum, sub)
			continue
		}

		// Determine effective flag namespace and (for new) kind.
		flagKey := sub
		newKind := ""
		if sub == "new" && len(tokens) >= 3 && !strings.HasPrefix(tokens[2], "-") {
			newKind = tokens[2]
			if !knownNewKinds[newKind] {
				t.Errorf("%s:%d: `docops new` unknown kind %q (must be ctx|adr|task)",
					filename, lineNum, newKind)
			}
			flagKey = "new/" + newKind
		}

		allowed, ok := flagAllowlist[flagKey]
		if !ok {
			// Fall back to generic "new" allowlist.
			allowed = flagAllowlist["new"]
		}

		// Check every long flag in the line.
		for _, tok := range tokens[2:] {
			if !strings.HasPrefix(tok, "--") {
				continue
			}
			// Strip =value if present.
			flag := tok
			if idx := strings.IndexByte(flag, '='); idx >= 0 {
				flag = flag[:idx]
			}
			if !allowed[flag] {
				t.Errorf("%s:%d: flag %q not in allowlist for `docops %s` (new kind=%q)",
					filename, lineNum, flag, sub, newKind)
			}
		}
	}
}
