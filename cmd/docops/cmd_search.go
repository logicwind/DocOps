package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/logicwind/docops/internal/index"
	"github.com/logicwind/docops/internal/schema"
)

// stringList is a repeatable flag value (used for --tag).
type stringList []string

func (s *stringList) String() string  { return strings.Join(*s, ", ") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

// searchResult is one match returned by cmdSearch.
type searchResult struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Status     string `json:"status,omitempty"`
	Snippet    string `json:"snippet"`
	MatchField string `json:"match_field"` // "title", "tags", "body", or ""

	// unexported ranking keys
	rank        int    // 1=title 2=tags 3=body-first-para 4=body-later 0=filter-only
	lastTouched string
}

// cmdSearch implements `docops search <query> [flags] [--json]`.
// Exit codes:
//
//	0  always (no matches is not an error)
//	2  bootstrap error, bad usage, or invalid regex
func cmdSearch(args []string) int {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON        := fs.Bool("json", false, "emit matches as JSON array")
	useRegex      := fs.Bool("regex", false, "treat query as a regexp pattern")
	caseSensitive := fs.Bool("case", false, "case-sensitive match (default is case-insensitive)")
	kindFlag      := fs.String("kind", "", "filter by kind: CTX, ADR, TP")
	statusFlag    := fs.String("status", "", "filter by status (per-kind)")
	coverage      := fs.String("coverage", "", "filter ADRs by coverage: required, not-needed")
	var tags stringList
	fs.Var(&tags, "tag", "filter ADRs by tag (repeatable; all tags must match)")
	priority  := fs.String("priority", "", "filter Tasks by priority: p0, p1, p2")
	assignee  := fs.String("assignee", "", "filter Tasks by assignee")
	since     := fs.String("since", "", "filter by last_touched >= YYYY-MM-DD")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops search [<query>] [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	query := ""
	if fs.NArg() > 0 {
		query = strings.Join(fs.Args(), " ")
	}

	kindUpper := strings.ToUpper(*kindFlag)

	// Guard: empty query requires at least one structured filter.
	hasFilter := kindUpper != "" || *statusFlag != "" || *coverage != "" ||
		len(tags) > 0 || *priority != "" || *assignee != "" || *since != ""
	if query == "" && !hasFilter {
		fmt.Fprintln(os.Stderr, "docops search: provide a query or at least one filter flag")
		fs.Usage()
		return 2
	}

	// Guard: kind-incompatible filter combos.
	if kindUpper == "CTX" && *statusFlag != "" {
		fmt.Fprintln(os.Stderr, "docops search: --status is not valid for --kind CTX (CTX has no status field)")
		return 2
	}
	if *coverage != "" && kindUpper != "" && kindUpper != "ADR" {
		fmt.Fprintln(os.Stderr, "docops search: --coverage is only valid for ADR")
		return 2
	}

	// Compile matcher.
	m, err := newMatcher(query, *useRegex, *caseSensitive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops search: invalid regex %q: %v\n", query, err)
		return 2
	}

	// Parse --since.
	sinceStr := *since // YYYY-MM-DD prefix comparison against RFC3339

	idx, root, code := bootstrapIndex("search")
	if code != 0 {
		return code
	}

	var results []searchResult
	for _, doc := range idx.Docs {
		// Structured filters.
		if kindUpper != "" && doc.Kind != kindUpper {
			continue
		}
		if *statusFlag != "" && docStatusField(doc) != *statusFlag {
			continue
		}
		if *coverage != "" && doc.ADRCoverage != *coverage {
			continue
		}
		for _, t := range tags {
			if !sliceContains(doc.ADRTags, t) {
				goto nextDoc
			}
		}
		if *priority != "" && doc.TaskPriority != *priority {
			continue
		}
		if *assignee != "" && doc.TaskAssignee != *assignee {
			continue
		}
		if sinceStr != "" && doc.LastTouched[:10] < sinceStr {
			continue
		}

		// Text match (skip if no query → filter-only, include all).
		if query == "" {
			results = append(results, searchResult{
				ID: doc.ID, Path: doc.Path, Kind: doc.Kind, Title: doc.CTXTitle,
				Status:      docStatusField(doc),
				Snippet:     doc.Summary,
				MatchField:  "",
				rank:        0,
				lastTouched: doc.LastTouched,
			})
			continue
		}

		if r := matchDoc(doc, root, m); r != nil {
			results = append(results, *r)
		}
	nextDoc:
	}

	// Sort: rank asc, lastTouched desc, id asc.
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].rank != results[j].rank {
			return results[i].rank < results[j].rank
		}
		if results[i].lastTouched != results[j].lastTouched {
			return results[i].lastTouched > results[j].lastTouched
		}
		return results[i].ID < results[j].ID
	})

	if *asJSON {
		out := make([]searchResult, len(results))
		copy(out, results)
		if out == nil {
			out = []searchResult{}
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "docops search: encode: %v\n", err)
			return 2
		}
		fmt.Println(string(b))
		return 0
	}

	for _, r := range results {
		kindStatus := r.Kind
		if r.Status != "" {
			kindStatus += "/" + r.Status
		}
		fmt.Printf("%s  %-14s  %s\n", r.ID, kindStatus, r.Title)
		if r.Snippet != "" {
			fmt.Printf("  (%s)  %s\n", fieldLabel(r.MatchField), r.Snippet)
		}
	}
	fmt.Printf("%d match(es)\n", len(results))
	return 0
}

// matcher encapsulates the text-match logic.
type matcher struct {
	query         string
	re            *regexp.Regexp
	useRegex      bool
	caseSensitive bool
}

func newMatcher(query string, useRegex, caseSensitive bool) (*matcher, error) {
	if query == "" {
		return &matcher{}, nil
	}
	m := &matcher{query: query, useRegex: useRegex, caseSensitive: caseSensitive}
	if useRegex {
		pattern := query
		if !caseSensitive {
			pattern = "(?i)" + pattern
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		m.re = re
	}
	return m, nil
}

// findIn returns the byte position of the first match in text, or -1.
func (m *matcher) findIn(text string) int {
	if m.query == "" {
		return -1
	}
	if m.useRegex {
		loc := m.re.FindStringIndex(text)
		if loc == nil {
			return -1
		}
		return loc[0]
	}
	q, t := m.query, text
	if !m.caseSensitive {
		q = strings.ToLower(q)
		t = strings.ToLower(t)
	}
	return strings.Index(t, q)
}

// matchDoc tries to match doc against the matcher, reading the body lazily.
// Returns nil if no match.
func matchDoc(doc index.IndexedDoc, root string, m *matcher) *searchResult {
	base := searchResult{
		ID: doc.ID, Path: doc.Path, Kind: doc.Kind, Title: doc.CTXTitle,
		Status:      docStatusField(doc),
		lastTouched: doc.LastTouched,
	}

	// 1. Title.
	if pos := m.findIn(doc.CTXTitle); pos >= 0 {
		r := base
		r.Snippet = doc.CTXTitle
		r.MatchField = "title"
		r.rank = 1
		return &r
	}

	// 2. Tags (any tag containing the query).
	for _, tag := range doc.ADRTags {
		if m.findIn(tag) >= 0 {
			r := base
			r.Snippet = strings.Join(doc.ADRTags, ", ")
			r.MatchField = "tags"
			r.rank = 2
			return &r
		}
	}

	// 3. Body (lazy read).
	raw, err := os.ReadFile(filepath.Join(root, doc.Path))
	if err != nil {
		return nil
	}
	_, bodyBytes, err := schema.SplitFrontmatter(raw)
	if err != nil {
		bodyBytes = raw
	}
	body := string(bodyBytes)

	pos := m.findIn(body)
	if pos < 0 {
		return nil
	}

	r := base
	r.MatchField = "body"
	r.Snippet = extractSnippet(body, pos, 120)
	if isFirstParagraph(body, pos) {
		r.rank = 3
	} else {
		r.rank = 4
	}
	return &r
}

// isFirstParagraph reports whether pos falls within the first paragraph
// (content before the first blank line).
func isFirstParagraph(body string, pos int) bool {
	end := strings.Index(body, "\n\n")
	if end < 0 {
		return true
	}
	return pos < end
}

// extractSnippet extracts ~maxLen chars centred on matchPos, with "…" padding.
func extractSnippet(body string, matchPos, maxLen int) string {
	half := maxLen / 2
	start := matchPos - half
	if start < 0 {
		start = 0
	}
	end := matchPos + half
	if end > len(body) {
		end = len(body)
	}
	// Snap to valid rune boundaries.
	for start > 0 && !utf8.RuneStart(body[start]) {
		start--
	}
	for end < len(body) && !utf8.RuneStart(body[end]) {
		end++
	}

	snippet := body[start:end]
	snippet = strings.ReplaceAll(snippet, "\n", " ")
	snippet = strings.Join(strings.Fields(snippet), " ") // collapse whitespace
	snippet = strings.TrimSpace(snippet)

	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(body) {
		snippet += "…"
	}
	return snippet
}

func fieldLabel(f string) string {
	if f == "" {
		return "filter"
	}
	return f
}
