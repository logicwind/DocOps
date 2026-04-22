package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/logicwind/docops/internal/index"
)

type nextOutput struct {
	index.IndexedDoc
	Reason string `json:"reason"`
}

// cmdNext implements `docops next [--assignee <name>] [--priority p0|p1|p2] [--json]`.
// Selection rules (first match wins):
//  1. Active tasks matching filters, sorted by last_touched descending.
//  2. Unblocked backlog tasks (all depends_on targets are done), sorted by
//     priority (p0>p1>p2) then ID ascending.
//
// Exit codes:
//
//	0  task found
//	1  no task matches
//	2  bootstrap error or bad usage
func cmdNext(args []string) int {
	fs := flag.NewFlagSet("next", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON    := fs.Bool("json", false, "emit the selected task as JSON")
	assignee  := fs.String("assignee", "", "filter by assignee")
	priority  := fs.String("priority", "", "filter by priority: p0, p1, p2")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops next [--assignee <name>] [--priority <p0|p1|p2>] [--json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	idx, code := bootstrapIndex("next")
	if code != 0 {
		return code
	}

	doneIDs := map[string]bool{}
	for _, doc := range idx.Docs {
		if doc.Kind == "TP" && doc.TaskStatus == "done" {
			doneIDs[doc.ID] = true
		}
	}

	matchesFilters := func(doc index.IndexedDoc) bool {
		if *assignee != "" && doc.TaskAssignee != *assignee {
			return false
		}
		if *priority != "" && doc.TaskPriority != *priority {
			return false
		}
		return true
	}

	// Rule 1: active tasks.
	var active []index.IndexedDoc
	for _, doc := range idx.Docs {
		if doc.Kind == "TP" && doc.TaskStatus == "active" && matchesFilters(doc) {
			active = append(active, doc)
		}
	}
	sort.Slice(active, func(i, j int) bool {
		// Most recently touched first.
		return active[i].LastTouched > active[j].LastTouched
	})
	if len(active) > 0 {
		t := active[0]
		assigneeName := t.TaskAssignee
		if assigneeName == "" {
			assigneeName = "unassigned"
		}
		return emitNext(t, "active for "+assigneeName, *asJSON)
	}

	// Rule 2+3: unblocked backlog tasks.
	var unblocked []index.IndexedDoc
	for _, doc := range idx.Docs {
		if doc.Kind != "TP" || doc.TaskStatus != "backlog" {
			continue
		}
		if !matchesFilters(doc) {
			continue
		}
		allDone := true
		for _, dep := range doc.TaskDependsOn {
			if !doneIDs[dep] {
				allDone = false
				break
			}
		}
		if allDone {
			unblocked = append(unblocked, doc)
		}
	}

	// Rule 4: priority (p0>p1>p2) then ID ascending.
	sort.Slice(unblocked, func(i, j int) bool {
		pi := priorityOrder(unblocked[i].TaskPriority)
		pj := priorityOrder(unblocked[j].TaskPriority)
		if pi != pj {
			return pi < pj
		}
		return unblocked[i].ID < unblocked[j].ID
	})
	if len(unblocked) > 0 {
		t := unblocked[0]
		reason := "unblocked backlog, " + t.TaskPriority
		return emitNext(t, reason, *asJSON)
	}

	fmt.Fprintln(os.Stderr, "docops next: no task matches")
	return 1
}

func emitNext(task index.IndexedDoc, reason string, asJSON bool) int {
	if asJSON {
		out := nextOutput{IndexedDoc: task, Reason: reason}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "docops next: encode: %v\n", err)
			return 2
		}
		fmt.Println(string(b))
		return 0
	}
	reqs := strings.Join(task.TaskRequires, ", ")
	if reqs == "" {
		reqs = "(none)"
	}
	fmt.Printf("%s (%s, %s) %s — requires: %s\n",
		task.ID, task.TaskAssignee, task.TaskPriority, task.CTXTitle, reqs)
	fmt.Printf("reason: %s\n", reason)
	return 0
}

func priorityOrder(p string) int {
	switch p {
	case "p0":
		return 0
	case "p1":
		return 1
	case "p2":
		return 2
	}
	return 99
}
