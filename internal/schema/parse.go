package schema

import (
	"bytes"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ErrNoFrontmatter is returned when a document does not begin with a `---`
// fenced YAML frontmatter block.
var ErrNoFrontmatter = errors.New("document must start with `---` frontmatter")

// ErrUnterminatedFrontmatter is returned when the opening `---` is not
// matched by a closing `---` on its own line.
var ErrUnterminatedFrontmatter = errors.New("unterminated frontmatter block (missing closing `---`)")

// SplitFrontmatter returns the YAML frontmatter bytes and the remaining
// body bytes from a markdown document. The returned frontmatter does not
// include the fence lines.
func SplitFrontmatter(raw []byte) (frontmatter, body []byte, err error) {
	// Tolerate a leading BOM and CRLF line endings.
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})

	const fence = "---"
	lines := bytes.SplitN(raw, []byte("\n"), 2)
	if len(lines) < 2 || !bytes.Equal(bytes.TrimRight(lines[0], "\r"), []byte(fence)) {
		return nil, nil, ErrNoFrontmatter
	}

	rest := lines[1]
	// Find the closing fence: a line containing only "---".
	var fmBuf bytes.Buffer
	for {
		nl := bytes.IndexByte(rest, '\n')
		var line []byte
		if nl < 0 {
			line = rest
			rest = nil
		} else {
			line = rest[:nl]
			rest = rest[nl+1:]
		}
		if bytes.Equal(bytes.TrimRight(line, "\r"), []byte(fence)) {
			return fmBuf.Bytes(), rest, nil
		}
		fmBuf.Write(line)
		fmBuf.WriteByte('\n')
		if nl < 0 {
			return nil, nil, ErrUnterminatedFrontmatter
		}
	}
}

// ParseContext decodes Context frontmatter from raw YAML bytes.
func ParseContext(fm []byte) (Context, error) {
	var c Context
	if err := strictUnmarshal(fm, &c); err != nil {
		return Context{}, fmt.Errorf("parse context frontmatter: %w", err)
	}
	return c, nil
}

// ParseADR decodes ADR frontmatter from raw YAML bytes.
func ParseADR(fm []byte) (ADR, error) {
	var a ADR
	if err := strictUnmarshal(fm, &a); err != nil {
		return ADR{}, fmt.Errorf("parse adr frontmatter: %w", err)
	}
	return a, nil
}

// ParseTask decodes Task frontmatter from raw YAML bytes.
func ParseTask(fm []byte) (Task, error) {
	var t Task
	if err := strictUnmarshal(fm, &t); err != nil {
		return Task{}, fmt.Errorf("parse task frontmatter: %w", err)
	}
	return t, nil
}

// strictUnmarshal uses yaml.v3's KnownFields to reject unknown keys so that
// typos in frontmatter surface as errors, not silent data loss.
func strictUnmarshal(data []byte, into any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(into); err != nil {
		return err
	}
	return nil
}
