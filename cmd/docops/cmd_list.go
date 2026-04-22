package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/logicwind/docops/internal/index"
)

// listRecord is the trimmed per-doc view emitted by `docops list`.
type listRecord struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Status         string `json:"status,omitempty"`
	Title          string `json:"title"`
	Coverage       string `json:"coverage,omitempty"`
	Priority       string `json:"priority,omitempty"`
	Assignee       string `json:"assignee,omitempty"`
	LastTouched    string `json:"last_touched"`
	Stale          bool   `json:"stale"`
	Implementation string `json:"implementation,omitempty"`
}

// cmdList implements `docops list [flags] [--json]`.
// Exit codes:
//
//	0  success (even if no docs match)
//	2  bootstrap error or bad usage
func cmdList(args []string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON    := fs.Bool("json", false, "emit records as JSON array")
	kindFlag  := fs.String("kind", "", "filter by kind: CTX, ADR, TP")
	statusFlag := fs.String("status", "", "filter by status (per-kind semantics)")
	coverage  := fs.String("coverage", "", "filter ADRs by coverage: required, not-needed")
	tag       := fs.String("tag", "", "filter ADRs by tag")
	onlyStale := fs.Bool("stale", false, "only docs with stale=true")
	since     := fs.String("since", "", "only docs with last_touched >= YYYY-MM-DD")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops list [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	idx, code := bootstrapIndex("list")
	if code != 0 {
		return code
	}

	var sinceTime time.Time
	if *since != "" {
		t, err := time.Parse("2006-01-02", *since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "docops list: --since %q: expected YYYY-MM-DD\n", *since)
			return 2
		}
		sinceTime = t
	}

	kindUpper := strings.ToUpper(*kindFlag)
	var records []listRecord
	for _, doc := range idx.Docs {
		if kindUpper != "" && doc.Kind != kindUpper {
			continue
		}
		docStatus := docStatusField(doc)
		if *statusFlag != "" && docStatus != *statusFlag {
			continue
		}
		if *coverage != "" && doc.ADRCoverage != *coverage {
			continue
		}
		if *tag != "" && !sliceContains(doc.ADRTags, *tag) {
			continue
		}
		if *onlyStale && !doc.Stale {
			continue
		}
		if !sinceTime.IsZero() {
			lt, err := time.Parse(time.RFC3339, doc.LastTouched)
			if err == nil && lt.Before(sinceTime) {
				continue
			}
		}
		records = append(records, listRecord{
			ID:             doc.ID,
			Kind:           doc.Kind,
			Status:         docStatus,
			Title:          doc.CTXTitle,
			Coverage:       doc.ADRCoverage,
			Priority:       doc.TaskPriority,
			Assignee:       doc.TaskAssignee,
			LastTouched:    doc.LastTouched,
			Stale:          doc.Stale,
			Implementation: doc.Implementation,
		})
	}

	// Sort: kind order (CTX→ADR→TP) then ID ascending.
	sort.SliceStable(records, func(i, j int) bool {
		ki := kindOrder(records[i].Kind)
		kj := kindOrder(records[j].Kind)
		if ki != kj {
			return ki < kj
		}
		return records[i].ID < records[j].ID
	})

	if *asJSON {
		if records == nil {
			records = []listRecord{}
		}
		b, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "docops list: encode: %v\n", err)
			return 2
		}
		fmt.Println(string(b))
		return 0
	}

	if len(records) == 0 {
		fmt.Println("(no documents match)")
		return 0
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tKIND\tSTATUS\tTITLE\tCOV/PRI\tIMPL/ASSIGNEE\tLAST-TOUCHED\tSTALE")
	for _, r := range records {
		extra1 := r.Coverage
		if extra1 == "" {
			extra1 = r.Priority
		}
		extra2 := r.Implementation
		if extra2 == "" {
			extra2 = r.Assignee
		}
		lt := r.LastTouched
		if len(lt) >= 10 {
			lt = lt[:10]
		}
		staleStr := "no"
		if r.Stale {
			staleStr = "yes"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.ID, r.Kind, r.Status, truncate(r.Title, 40),
			extra1, extra2, lt, staleStr)
	}
	_ = tw.Flush()
	return 0
}

// docStatusField returns the kind-appropriate status string (empty for CTX).
func docStatusField(doc index.IndexedDoc) string {
	switch doc.Kind {
	case "ADR":
		return doc.ADRStatus
	case "TP":
		return doc.TaskStatus
	}
	return ""
}

func kindOrder(k string) int {
	switch k {
	case "CTX":
		return 0
	case "ADR":
		return 1
	case "TP":
		return 2
	}
	return 3
}

// sliceContains reports whether s appears in sl.
func sliceContains(sl []string, s string) bool {
	for _, v := range sl {
		if v == s {
			return true
		}
	}
	return false
}

// truncate clips s to maxLen runes, appending "…" if truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}
