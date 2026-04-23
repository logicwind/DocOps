package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/logicwind/docops/internal/index"
)

// cmdGet implements `docops get <id> [--json]`.
// Exit codes:
//
//	0  found
//	1  not found
//	2  bootstrap error or bad usage
func cmdGet(args []string) int {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "emit the full IndexedDoc as JSON")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops get <id> [--json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "docops get: requires exactly one argument <id>")
		fs.Usage()
		return 2
	}
	id := fs.Arg(0)

	idx, _, code := bootstrapIndex("get")
	if code != 0 {
		return code
	}

	doc, ok := indexLookup(idx, id)
	if !ok {
		fmt.Fprintf(os.Stderr, "docops get: %q not found\n", id)
		return 1
	}

	if *asJSON {
		b, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "docops get: encode: %v\n", err)
			return 2
		}
		fmt.Println(string(b))
		return 0
	}

	fmt.Print(humanGet(doc))
	return 0
}

func humanGet(doc index.IndexedDoc) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  %s\n", doc.ID, doc.CTXTitle)

	field := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&sb, "  %-16s %s\n", k+":", v)
		}
	}

	field("kind", doc.Kind)
	switch doc.Kind {
	case "ADR":
		field("status", doc.ADRStatus)
		field("coverage", doc.ADRCoverage)
		field("date", doc.ADRDate)
		if len(doc.ADRTags) > 0 {
			field("tags", strings.Join(doc.ADRTags, ", "))
		}
		if len(doc.CTXSupersedes) > 0 {
			field("supersedes", strings.Join(doc.CTXSupersedes, ", "))
		}
		if len(doc.ADRRelated) > 0 {
			field("related", strings.Join(doc.ADRRelated, ", "))
		}
		field("implementation", doc.Implementation)
	case "TP":
		field("status", doc.TaskStatus)
		field("priority", doc.TaskPriority)
		field("assignee", doc.TaskAssignee)
		if len(doc.TaskRequires) > 0 {
			field("requires", strings.Join(doc.TaskRequires, ", "))
		}
		if len(doc.TaskDependsOn) > 0 {
			field("depends_on", strings.Join(doc.TaskDependsOn, ", "))
		}
	case "CTX":
		field("type", doc.CTXType)
		if len(doc.CTXSupersedes) > 0 {
			field("supersedes", strings.Join(doc.CTXSupersedes, ", "))
		}
	}

	field("summary", doc.Summary)
	lt := doc.LastTouched
	if len(lt) >= 10 {
		lt = lt[:10]
	}
	field("last_touched", lt)
	fmt.Fprintf(&sb, "  %-16s %d\n", "age_days:", doc.AgeDays)
	fmt.Fprintf(&sb, "  %-16s %d\n", "word_count:", doc.WordCount)
	staleStr := "false"
	if doc.Stale {
		staleStr = "true"
	}
	field("stale", staleStr)

	// Reverse edges.
	if len(doc.SupersededBy) > 0 {
		field("superseded_by", strings.Join(doc.SupersededBy, ", "))
	}
	if len(doc.ReferencedBy) > 0 {
		parts := make([]string, len(doc.ReferencedBy))
		for i, r := range doc.ReferencedBy {
			parts[i] = fmt.Sprintf("%s (%s)", r.ID, r.Edge)
		}
		field("referenced_by", strings.Join(parts, ", "))
	}
	if len(doc.ActiveTasks) > 0 {
		field("active_tasks", strings.Join(doc.ActiveTasks, ", "))
	}
	if len(doc.DerivedADRs) > 0 {
		field("derived_adrs", strings.Join(doc.DerivedADRs, ", "))
	}
	if len(doc.Blocks) > 0 {
		field("blocks", strings.Join(doc.Blocks, ", "))
	}

	return sb.String()
}
