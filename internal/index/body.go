package index

import (
	"bytes"
	"regexp"
	"strings"
	"unicode"
)

// summaryOverrideRE matches an HTML comment like <!-- summary: the text here -->
// placed anywhere in the body.
var summaryOverrideRE = regexp.MustCompile(`<!--\s*summary:\s*(.*?)\s*-->`)

// headingRE matches markdown heading lines.
var headingRE = regexp.MustCompile(`^#{1,6}\s`)

// codeFenceRE matches ``` or ~~~ fence openers/closers.
var codeFenceRE = regexp.MustCompile("^(```|~~~)")

const summaryMaxLen = 200

// bodySummary derives the display summary and an approximate word count from
// the raw body bytes (everything after the frontmatter closing `---`).
//
// Summary rules (in priority order):
//  1. If the body contains <!-- summary: text --> use that text.
//  2. Otherwise, skip leading heading lines and return the first non-empty
//     paragraph (lines up to the first blank line), trimmed and capped at
//     summaryMaxLen chars with an ellipsis.
//
// Word count: split on whitespace after stripping code-fence blocks.
func bodySummary(body []byte) (summary string, wordCount int) {
	text := string(body)

	// Summary override via HTML comment.
	if m := summaryOverrideRE.FindStringSubmatch(text); m != nil {
		summary = strings.TrimSpace(m[1])
	}

	wordCount = countWords(stripCodeFences(text))

	if summary != "" {
		return summary, wordCount
	}

	summary = firstParagraph(text)
	return summary, wordCount
}

// firstParagraph returns the first prose paragraph: skip blank lines, skip
// heading-only lines, collect lines until the next blank, join, trim, and
// cap at summaryMaxLen.
func firstParagraph(text string) string {
	lines := strings.Split(text, "\n")
	var para []string
	collecting := false

	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")

		if !collecting {
			// Skip blank lines and headings before the first paragraph.
			if trimmed == "" || headingRE.MatchString(trimmed) {
				continue
			}
			collecting = true
		}

		// Blank line ends the paragraph.
		if trimmed == "" {
			break
		}
		para = append(para, trimmed)
	}

	if len(para) == 0 {
		return ""
	}

	joined := strings.Join(para, " ")
	joined = strings.TrimSpace(joined)
	if len(joined) > summaryMaxLen {
		// Truncate on a rune boundary and append ellipsis.
		runes := []rune(joined)
		if len(runes) > summaryMaxLen {
			joined = string(runes[:summaryMaxLen]) + "…"
		}
	}
	return joined
}

// stripCodeFences removes content between ``` or ~~~ fence pairs so that
// code tokens don't inflate the word count.
func stripCodeFences(text string) string {
	var buf bytes.Buffer
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		if codeFenceRE.MatchString(strings.TrimLeft(line, " \t")) {
			inFence = !inFence
			continue
		}
		if !inFence {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	return buf.String()
}

// countWords counts whitespace-delimited tokens in s.
func countWords(s string) int {
	n := 0
	inWord := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			inWord = false
		} else {
			if !inWord {
				n++
			}
			inWord = true
		}
	}
	return n
}
