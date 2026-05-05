// Package scaffold contains filesystem-action helpers shared by the
// scaffolding commands (`docops init`, `docops upgrade`). It is a pure
// utility layer: planning a write, deciding whether a file is up to
// date, merging the AGENTS.md docops block, and applying actions to
// disk. Higher-level packages (initter, upgrader) compose these
// primitives into command-specific behavior.
package scaffold

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/logicwind/docops/templates"
)

// Action kinds. Kept as string constants (not a typed enum) so JSON
// output and human-readable diffs share one vocabulary.
const (
	KindMkdir       = "mkdir"
	KindWriteFile   = "write-file"
	KindMergeAgents = "merge-agents"
	KindRemove      = "remove"
	KindSkip        = "skip"
)

// AGENTS.md docops block delimiters. Mirrors the defaults in the
// shipped docops.yaml template so init and upgrade agree on what the
// "managed region" looks like.
const (
	BlockStart = "<!-- docops:start -->"
	BlockEnd   = "<!-- docops:end -->"
)

// Action is a single filesystem change proposed by a planner. Planners
// build the full action set first, then execute it in one pass so
// dry-run and the real write share the same code path.
type Action struct {
	Path   string      // absolute destination
	Rel    string      // Path relative to the project root, used in human output
	Kind   string      // one of the Kind* constants
	Reason string      // why the action is a skip, write, or overwrite
	Body   []byte      // bytes to write (empty for mkdir / skip)
	Mode   os.FileMode // 0o755 for the pre-commit hook, 0o644 otherwise
}

// DirAction builds a mkdir action that becomes a skip when the
// directory already exists. Keeps idempotent re-runs quiet.
func DirAction(rootAbs, rel string) Action {
	abs := filepath.Join(rootAbs, rel)
	if info, err := os.Stat(abs); err == nil && info.IsDir() {
		return Action{
			Path:   abs,
			Rel:    rel,
			Kind:   KindSkip,
			Reason: "directory exists",
			Mode:   0o755,
		}
	}
	return Action{
		Path:   abs,
		Rel:    rel,
		Kind:   KindMkdir,
		Reason: "create directory",
		Mode:   0o755,
	}
}

// FileAction builds a write-file action, deciding whether the target
// should be created, overwritten (force=true on drift), or skipped.
// A read error other than ENOENT is folded into the action's Reason
// and surfaces during Execute.
func FileAction(rootAbs, rel string, body []byte, mode os.FileMode, force bool) Action {
	abs := filepath.Join(rootAbs, rel)
	a := Action{Path: abs, Rel: rel, Kind: KindWriteFile, Body: body, Mode: mode}
	existing, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			a.Reason = "create"
			return a
		}
		a.Reason = "create (read failed: " + err.Error() + ")"
		return a
	}
	if bytes.Equal(existing, body) {
		a.Kind = KindSkip
		a.Reason = "already up to date"
		return a
	}
	if force {
		a.Reason = "overwrite drifted content (--force)"
		return a
	}
	a.Kind = KindSkip
	a.Reason = "exists and differs — rerun with --force to overwrite"
	return a
}

// MergeAgentsBlock replaces the docops:start/end block inside existing
// content with the block extracted from tmpl. If no block is present
// in existing, the template block is appended. Returns the merged
// bytes, a changed flag (false = identical to existing), and a reason
// string for human output.
func MergeAgentsBlock(existing, tmpl []byte) ([]byte, bool, string) {
	tmplBlock := ExtractBlock(tmpl)
	if tmplBlock == "" {
		return existing, false, "template missing docops block; skipping merge"
	}

	ex := string(existing)
	startIdx := strings.Index(ex, BlockStart)
	endIdx := strings.Index(ex, BlockEnd)
	if startIdx >= 0 && endIdx > startIdx {
		endIdx += len(BlockEnd)
		replacement := BlockStart + tmplBlock + BlockEnd
		merged := ex[:startIdx] + replacement + ex[endIdx:]
		if merged == ex {
			return existing, false, "docops block already up to date"
		}
		return []byte(merged), true, "refresh docops block"
	}

	appended := ex
	if !strings.HasSuffix(appended, "\n") {
		appended += "\n"
	}
	appended += "\n" + BlockStart + tmplBlock + BlockEnd + "\n"
	return []byte(appended), true, "append docops block to existing AGENTS.md"
}

// ExtractBlock returns the content between BlockStart and BlockEnd in
// tmpl. Returns "" if either marker is missing. Leading/trailing
// newlines are preserved so the rendered block matches the template
// byte-for-byte.
func ExtractBlock(tmpl []byte) string {
	s := string(tmpl)
	start := strings.Index(s, BlockStart)
	if start < 0 {
		return ""
	}
	start += len(BlockStart)
	end := strings.Index(s, BlockEnd)
	if end < 0 || end < start {
		return ""
	}
	return s[start:end]
}

// Execute applies one planned action to the filesystem. Skips are
// no-ops; mkdir creates with 0o755; writes ensure parent dirs exist
// and use the action's Mode; remove deletes the file (a missing file
// is not an error).
func Execute(a *Action) error {
	switch a.Kind {
	case KindSkip:
		return nil
	case KindMkdir:
		return os.MkdirAll(a.Path, 0o755)
	case KindWriteFile, KindMergeAgents:
		if err := os.MkdirAll(filepath.Dir(a.Path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(a.Path, a.Body, a.Mode)
	case KindRemove:
		if err := os.Remove(a.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return fmt.Errorf("unknown action kind %q", a.Kind)
}

// PrintPlan writes a human-readable summary of an action set. label
// prefixes the summary line ("docops init", "docops upgrade", …).
// dry switches verbs from past tense to "would apply". Callers may
// append command-specific footers (next-steps, warnings) afterwards.
func PrintPlan(w io.Writer, actions []Action, dry bool, label string) {
	var changed, skipped int
	for _, a := range actions {
		if a.Kind == KindSkip {
			skipped++
		} else {
			changed++
		}
	}
	verb := "applied"
	if dry {
		verb = "would apply"
	}
	fmt.Fprintf(w, "%s: %s %d change(s), skipped %d\n", label, verb, changed, skipped)
	for _, a := range actions {
		tag := a.Kind
		if dry && a.Kind != KindSkip {
			tag = "+ " + a.Kind
		}
		fmt.Fprintf(w, "  %-13s %-34s %s\n", tag, a.Rel, a.Reason)
	}
}

// LoadShippedSkills returns the embedded skill files keyed by
// basename (e.g. "init.md" → bytes). Thin wrapper around
// templates.Skills() so initter and upgrader share one entry point.
func LoadShippedSkills() (map[string][]byte, error) {
	return templates.Skills()
}

// SlashDeliverableCmds returns the subset of shipped commands that ship
// as user-facing slash commands per ADR-0029. The full shipped set still
// ships as skills (NL-dispatched) and CLI verbs; this only narrows the
// slash-command surface.
//
// LayoutSkillBundle harnesses (Codex) deliver the full shipped set as
// in-bundle subroutines and bypass this filter — the bundle is the
// skill mechanism, not a slash mechanism.
//
// Update this set when adding or removing milestone-moment commands.
// `baseline` will be added when ADR-0030's implementation lands.
func SlashDeliverableCmds() map[string]bool {
	return map[string]bool{
		"init":     true,
		"onboard":  true,
		"progress": true,
		"next":     true,
		"do":       true,
		"plan":     true,
	}
}

// FilterSlashDeliverable returns the input list filtered to commands in
// SlashDeliverableCmds, preserving order. Used by initter and upgrader
// to narrow slash-style harness writes per ADR-0029.
func FilterSlashDeliverable(cmds []string) []string {
	keep := SlashDeliverableCmds()
	out := make([]string, 0, len(cmds))
	for _, c := range cmds {
		if keep[c] {
			out = append(out, c)
		}
	}
	return out
}
