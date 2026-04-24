package upgrader

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// parseFrontmatter splits a markdown document into its YAML frontmatter
// map and the raw body that follows. The body is the bytes after the
// closing "---" line (including any trailing newline on that line). If
// the document has no leading "---\n" block, fm is nil and body is the
// full document unchanged.
//
// The returned node is the parsed yaml.Node for the mapping block — used
// by applyTransform to preserve key order on round-trip.
func parseFrontmatter(src []byte) (fm map[string]any, node *yaml.Node, body []byte, err error) {
	s := string(src)
	if !strings.HasPrefix(s, "---\n") {
		return nil, nil, src, nil
	}
	// Find the closing "---" on its own line.
	rest := s[4:] // skip opening "---\n"
	idx := strings.Index(rest, "\n---")
	if idx == -1 {
		return nil, nil, src, nil
	}
	yamlText := rest[:idx]
	afterClose := rest[idx+4:] // skip "\n---"
	body = []byte(afterClose)

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlText), &doc); err != nil {
		return nil, nil, src, fmt.Errorf("frontmatter: parse yaml: %w", err)
	}

	// doc is a DocumentNode; its first child is the MappingNode.
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, nil, src, fmt.Errorf("frontmatter: unexpected yaml structure (kind=%d)", doc.Kind)
	}
	mappingNode := doc.Content[0]

	var parsed map[string]any
	if err := mappingNode.Decode(&parsed); err != nil {
		return nil, nil, src, fmt.Errorf("frontmatter: decode yaml: %w", err)
	}
	return parsed, mappingNode, body, nil
}

// serializeFrontmatter serializes a frontmatter map back to a YAML block
// and prepends it to body. It attempts to preserve the original key order
// from origNode: keys that survive in fm are emitted first in their
// original order, then any new keys are appended alphabetically.
// Keys removed by the transform are omitted.
//
// body is appended verbatim; it already includes the leading "\n" (or
// nothing if the source ended at "---").
func serializeFrontmatter(fm map[string]any, origNode *yaml.Node, body []byte) ([]byte, error) {
	// Build an ordered key list: surviving original keys first (in order),
	// then any new keys added by the transform (sorted for determinism).
	seen := make(map[string]bool, len(fm))
	var orderedKeys []string

	// origNode is a MappingNode: Content = [key0, val0, key1, val1, ...]
	if origNode != nil && origNode.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(origNode.Content); i += 2 {
			k := origNode.Content[i].Value
			if _, ok := fm[k]; ok {
				orderedKeys = append(orderedKeys, k)
				seen[k] = true
			}
		}
	}
	// Append new keys (present in fm but not in original).
	var newKeys []string
	for k := range fm {
		if !seen[k] {
			newKeys = append(newKeys, k)
		}
	}
	// Sort new keys for determinism.
	sortStrings(newKeys)
	orderedKeys = append(orderedKeys, newKeys...)

	// Serialize each key-value pair using yaml.Marshal for values.
	var buf bytes.Buffer
	buf.WriteString("---\n")
	for _, k := range orderedKeys {
		v := fm[k]
		valBytes, err := marshalYAMLValue(k, v)
		if err != nil {
			return nil, fmt.Errorf("frontmatter: marshal key %q: %w", k, err)
		}
		buf.Write(valBytes)
	}
	// Remove trailing newline before closing "---".
	out := bytes.TrimRight(buf.Bytes(), "\n")
	var result bytes.Buffer
	result.Write(out)
	result.WriteString("\n---")
	result.Write(body)
	return result.Bytes(), nil
}

// marshalYAMLValue serializes one key-value pair as YAML. For scalar
// values (string, bool, int, float) it emits "key: value\n". For
// sequences and maps it uses yaml.Marshal to get the block form.
func marshalYAMLValue(key string, value any) ([]byte, error) {
	// Use yaml.Marshal on a single-key map so we get proper indentation
	// for nested values (sequences, maps).
	m := map[string]any{key: value}
	raw, err := yaml.Marshal(m)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// applyTransform parses the frontmatter of src, runs it through
// transform, then re-serializes the result preserving key order.
// If src has no frontmatter the document is returned unchanged.
func applyTransform(src []byte, transform func(map[string]any) (map[string]any, error)) ([]byte, error) {
	fm, node, body, err := parseFrontmatter(src)
	if err != nil {
		return nil, err
	}
	if fm == nil {
		return src, nil
	}
	out, err := transform(fm)
	if err != nil {
		return nil, err
	}
	return serializeFrontmatter(out, node, body)
}

// sortStrings sorts a string slice in place (insertion sort for small slices).
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}
