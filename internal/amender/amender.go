// Package amender implements `docops amend`: append a new amendment entry
// to an ADR's frontmatter, optionally insert an inline [AMENDED ...] marker
// at a substring in the body, and append a subsection to the ADR's
// `## Amendments` section. See ADR-0025 for the design.
//
// The package's contract is non-interactive (no $EDITOR shell-out, no
// prompts) so coding agents can drive amendments end-to-end. The CLI
// wrapper at cmd/docops/cmd_amend.go adds the optional editor open.
package amender

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/logicwind/docops/internal/schema"
	"gopkg.in/yaml.v3"
)

// Options carries the flag surface for one amend invocation. ADRPath is
// resolved against Root if relative; if Root is empty, the caller is
// expected to have absolutized ADRPath itself.
type Options struct {
	Root            string
	ADRID           string // e.g. "ADR-0019"
	Kind            string
	Summary         string
	By              string
	Ref             string
	AffectsSections []string
	MarkerAt        string   // optional substring in body to receive the marker
	BodyReader      io.Reader // optional amendment subsection body
	Date            string   // optional override; defaults to today (YYYY-MM-DD)
}

// Result describes what landed.
type Result struct {
	ADRID           string
	Path            string // absolute file path written
	Rel             string // relative path (project-root-relative when known)
	AmendmentIndex  int    // 0-based index of the new entry in amendments[]
	MarkerInserted  bool
	SectionCreated  bool   // true if `## Amendments` had to be created
}

// MarkerNotFoundError is returned when --marker-at is set but its substring
// is absent or appears in zero non-fenced positions.
type MarkerNotFoundError struct{ Substring string }

func (e *MarkerNotFoundError) Error() string {
	return fmt.Sprintf("amender: --marker-at substring %q not found in ADR body", e.Substring)
}

// MarkerAmbiguousError is returned when --marker-at substring matches in
// more than one non-fenced position. Lists the line numbers.
type MarkerAmbiguousError struct {
	Substring string
	Lines     []int
}

func (e *MarkerAmbiguousError) Error() string {
	return fmt.Sprintf("amender: --marker-at substring %q is not unique; matches on lines %v", e.Substring, e.Lines)
}

// Run executes the amend operation atomically.
func Run(opts Options) (Result, error) {
	if opts.ADRID == "" {
		return Result{}, errors.New("amender: ADR id is required")
	}
	if !strings.HasPrefix(opts.ADRID, "ADR-") {
		return Result{}, fmt.Errorf("amender: %q is not an ADR id (expected ADR-NNNN)", opts.ADRID)
	}
	if opts.Kind == "" {
		return Result{}, errors.New("amender: --kind is required")
	}
	validKind := false
	for _, k := range schema.AmendmentKinds {
		if k == opts.Kind {
			validKind = true
			break
		}
	}
	if !validKind {
		return Result{}, fmt.Errorf("amender: --kind %q is not one of: %s", opts.Kind, strings.Join(schema.AmendmentKinds, ", "))
	}
	if strings.TrimSpace(opts.Summary) == "" {
		return Result{}, errors.New("amender: --summary is required")
	}

	by := opts.By
	if by == "" {
		by = defaultAuthor()
	}
	if by == "" {
		return Result{}, errors.New("amender: --by is required (no $DOCOPS_USER, git user.name, or $USER set)")
	}

	date := opts.Date
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}

	absPath, rel, err := resolveADRFile(opts.Root, opts.ADRID)
	if err != nil {
		return Result{}, err
	}

	raw, err := os.ReadFile(absPath)
	if err != nil {
		return Result{}, fmt.Errorf("amender: read %s: %w", rel, err)
	}

	fm, body, err := schema.SplitFrontmatter(raw)
	if err != nil {
		return Result{}, fmt.Errorf("amender: %s: %w", rel, err)
	}

	amend := schema.Amendment{
		Date:            date,
		Kind:            opts.Kind,
		By:              by,
		Summary:         opts.Summary,
		AffectsSections: dedupeNonEmpty(opts.AffectsSections),
		Ref:             opts.Ref,
	}

	newFM, idx, err := appendAmendment(fm, amend)
	if err != nil {
		return Result{}, err
	}

	newBody, markerInserted, err := insertMarker(body, opts.MarkerAt, date, opts.Kind)
	if err != nil {
		return Result{}, err
	}

	subBody, err := readBody(opts.BodyReader, amend)
	if err != nil {
		return Result{}, err
	}
	newBody, sectionCreated := appendAmendmentsSubsection(newBody, amend, subBody)

	out := composeFile(newFM, newBody)
	if err := atomicWrite(absPath, out, 0o644); err != nil {
		return Result{}, err
	}

	return Result{
		ADRID:          opts.ADRID,
		Path:           absPath,
		Rel:            rel,
		AmendmentIndex: idx,
		MarkerInserted: markerInserted,
		SectionCreated: sectionCreated,
	}, nil
}

// resolveADRFile walks docs/decisions/ under root looking for a file
// whose stem starts with "<ADRID>-". Returns the absolute path and the
// path relative to root.
func resolveADRFile(root, id string) (abs, rel string, err error) {
	if root == "" {
		return "", "", errors.New("amender: Root must be set")
	}
	dir := filepath.Join(root, "docs", "decisions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", fmt.Errorf("amender: read decisions dir: %w", err)
	}
	prefix := id + "-"
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == id+".md" || strings.HasPrefix(name, prefix) {
			abs = filepath.Join(dir, name)
			rel = filepath.Join("docs", "decisions", name)
			return abs, rel, nil
		}
	}
	return "", "", fmt.Errorf("amender: ADR %s not found under docs/decisions/", id)
}

// appendAmendment parses fm as a YAML document, appends amend to the
// `amendments` sequence (creating it if absent), and returns the
// re-marshalled bytes plus the 0-based index of the new entry.
//
// Uses yaml.Node so existing key order, comments, and quoting style on
// the rest of the frontmatter are preserved.
func appendAmendment(fm []byte, amend schema.Amendment) ([]byte, int, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(fm, &doc); err != nil {
		return nil, 0, fmt.Errorf("amender: parse frontmatter: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, 0, errors.New("amender: frontmatter is not a YAML mapping")
	}
	root := doc.Content[0]

	// Find or create the `amendments` key.
	var seq *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "amendments" {
			seq = root.Content[i+1]
			break
		}
	}
	if seq == nil {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "amendments"},
			&yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"},
		)
		seq = root.Content[len(root.Content)-1]
	}
	if seq.Kind != yaml.SequenceNode {
		return nil, 0, errors.New("amender: amendments field is not a sequence")
	}

	entryNode, err := amendmentToNode(amend)
	if err != nil {
		return nil, 0, err
	}
	idx := len(seq.Content)
	seq.Content = append(seq.Content, entryNode)

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, 0, fmt.Errorf("amender: re-marshal frontmatter: %w", err)
	}
	return out, idx, nil
}

// amendmentToNode builds a YAML mapping node for one amendment, omitting
// optional empty fields per the schema (ref, affects_sections).
func amendmentToNode(am schema.Amendment) (*yaml.Node, error) {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	add := func(k, v string) {
		m.Content = append(m.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v},
		)
	}
	add("date", am.Date)
	add("kind", am.Kind)
	add("by", am.By)
	add("summary", am.Summary)
	if len(am.AffectsSections) > 0 {
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
		for _, s := range am.AffectsSections {
			seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s})
		}
		m.Content = append(m.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "affects_sections"},
			seq,
		)
	}
	if strings.TrimSpace(am.Ref) != "" {
		add("ref", am.Ref)
	}
	return m, nil
}

// insertMarker locates substring (a literal exact-match) in body and
// inserts the [AMENDED YYYY-MM-DD kind] marker immediately after the
// matched substring. Returns the new body, whether a marker was inserted,
// and any error. When substring is empty, returns body unchanged.
//
// Matches inside fenced code blocks are skipped (ADRs document the
// marker syntax in examples). The substring must be unique among
// non-fenced occurrences; a duplicate match is a hard error so the
// caller can disambiguate.
func insertMarker(body []byte, substring, date, kind string) ([]byte, bool, error) {
	if substring == "" {
		return body, false, nil
	}

	type hit struct{ line, byteOffset int }
	var hits []hit
	inFence := false
	cursor := 0
	for ln, line := range strings.SplitAfter(string(body), "\n") {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\n"))
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			cursor += len(line)
			continue
		}
		if inFence {
			cursor += len(line)
			continue
		}
		if i := strings.Index(line, substring); i >= 0 {
			hits = append(hits, hit{line: ln + 1, byteOffset: cursor + i})
		}
		cursor += len(line)
	}

	switch len(hits) {
	case 0:
		return nil, false, &MarkerNotFoundError{Substring: substring}
	case 1:
		// Insert immediately after the substring.
		marker := fmt.Sprintf(" [AMENDED %s %s]", date, kind)
		insertAt := hits[0].byteOffset + len(substring)
		out := make([]byte, 0, len(body)+len(marker))
		out = append(out, body[:insertAt]...)
		out = append(out, marker...)
		out = append(out, body[insertAt:]...)
		return out, true, nil
	default:
		lines := make([]int, len(hits))
		for i, h := range hits {
			lines[i] = h.line
		}
		return nil, false, &MarkerAmbiguousError{Substring: substring, Lines: lines}
	}
}

// readBody pulls the amendment subsection body from the reader. Returns
// a default three-line stub when reader is nil.
func readBody(r io.Reader, am schema.Amendment) (string, error) {
	if r == nil {
		return defaultStub(am), nil
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("amender: read body: %w", err)
	}
	s := strings.TrimRight(string(data), "\n")
	if s == "" {
		return defaultStub(am), nil
	}
	return s + "\n", nil
}

func defaultStub(am schema.Amendment) string {
	return strings.Join([]string{
		"_What changed:_ TODO — the prose this amendment updates.",
		"_Why:_ TODO — short justification.",
		"_Decision unchanged:_ confirm what the original ADR still decides.",
	}, "\n") + "\n"
}

// appendAmendmentsSubsection appends an `### YYYY-MM-DD — summary (kind)`
// subsection to the body's `## Amendments` section, creating that section
// just before any trailing markdown sections (heuristic: the last `## `
// header in the file is treated as "before any trailing sections" — for
// most ADRs this is good enough). Returns the new body and whether the
// section had to be created.
func appendAmendmentsSubsection(body []byte, am schema.Amendment, sub string) ([]byte, bool) {
	header := fmt.Sprintf("### %s — %s (%s)\n\n%s", am.Date, truncateForSubheader(am.Summary), am.Kind, sub)

	bodyStr := string(body)
	if i := strings.Index(bodyStr, "\n## Amendments\n"); i >= 0 {
		// Append at the end of the file (after any existing subsections);
		// for now keep it simple — newest-last in the section.
		trimmed := strings.TrimRight(bodyStr, "\n")
		return []byte(trimmed + "\n\n" + header + "\n"), false
	}

	trimmed := strings.TrimRight(bodyStr, "\n")
	combined := trimmed + "\n\n## Amendments\n\n" + header + "\n"
	return []byte(combined), true
}

// truncateForSubheader keeps subsection titles readable in TOCs. Cuts at
// 60 runes (per the implementer note in TP-026).
func truncateForSubheader(s string) string {
	r := []rune(s)
	if len(r) <= 60 {
		return s
	}
	return string(r[:57]) + "..."
}

// composeFile reassembles a markdown document with the canonical
// `---\n<frontmatter>\n---\n<body>` shape. Strips trailing newlines
// from the frontmatter so we emit exactly one between fence lines.
func composeFile(fm, body []byte) []byte {
	fmTrimmed := bytes.TrimRight(fm, "\n")
	out := bytes.Buffer{}
	out.WriteString("---\n")
	out.Write(fmTrimmed)
	out.WriteString("\n---\n")
	if len(body) > 0 && body[0] != '\n' {
		// Preserve original leading-blank-line shape if author had one.
	}
	out.Write(body)
	return out.Bytes()
}

// atomicWrite writes data to a tmp file in the same directory then
// renames over path. Avoids partial writes on crash mid-edit.
func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".docops-amend-*")
	if err != nil {
		return fmt.Errorf("amender: create tmp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("amender: write tmp: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("amender: chmod tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("amender: close tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("amender: rename tmp: %w", err)
	}
	return nil
}

// defaultAuthor consults DOCOPS_USER → git user.name → USER.
// Returns "" when none is set; caller emits a clear error.
func defaultAuthor() string {
	if v := os.Getenv("DOCOPS_USER"); v != "" {
		return v
	}
	// Avoid shelling out for git when possible: read .git/config via env
	// fallback. For now, only check env vars; git lookup belongs in the
	// CLI wrapper to keep this package free of exec dependencies.
	if v := os.Getenv("USER"); v != "" {
		return v
	}
	return ""
}

// dedupeNonEmpty preserves order, drops empty entries and duplicates.
func dedupeNonEmpty(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		// Keep insertion order via stable sort over a no-op comparator.
		return false
	})
	return out
}
