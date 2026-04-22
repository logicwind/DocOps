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

type graphEdge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Edge      string `json:"edge"`
	Direction string `json:"direction"` // "outbound" or "inbound"
}

type graphOutput struct {
	Root  string             `json:"root"`
	Nodes []index.IndexedDoc `json:"nodes"`
	Edges []graphEdge        `json:"edges"`
}

// cmdGraph implements `docops graph <id> [--depth N] [--json]`.
// Exit codes:
//
//	0  success
//	1  ID not found
//	2  bootstrap error or bad usage
func cmdGraph(args []string) int {
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "emit graph as JSON")
	depth := fs.Int("depth", 1, "traversal depth")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: docops graph <id> [--depth N] [--json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "docops graph: requires exactly one argument <id>")
		fs.Usage()
		return 2
	}
	id := fs.Arg(0)
	if *depth < 0 {
		*depth = 0
	}

	idx, _, code := bootstrapIndex("graph")
	if code != 0 {
		return code
	}

	byID := indexByID(idx)
	if _, ok := byID[id]; !ok {
		fmt.Fprintf(os.Stderr, "docops graph: %q not found\n", id)
		return 1
	}

	nodes, edges := walkGraph(id, byID, *depth)

	if *asJSON {
		out := graphOutput{Root: id, Nodes: nodes, Edges: edges}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "docops graph: encode: %v\n", err)
			return 2
		}
		fmt.Println(string(b))
		return 0
	}

	fmt.Print(humanGraph(id, byID, edges, *depth))
	return 0
}

// walkGraph performs a BFS from startID up to maxDepth hops, collecting
// visited nodes and the edges between them. Cycle-safe via visited set.
func walkGraph(startID string, byID map[string]index.IndexedDoc, maxDepth int) ([]index.IndexedDoc, []graphEdge) {
	type qItem struct {
		id    string
		depth int
	}

	visited := map[string]struct{}{}
	edgeSeen := map[string]struct{}{}
	var nodes []index.IndexedDoc
	var edges []graphEdge
	queue := []qItem{{startID, 0}}

	addEdge := func(from, to, label, dir string) {
		key := from + "|" + to + "|" + label + "|" + dir
		if _, seen := edgeSeen[key]; seen {
			return
		}
		edgeSeen[key] = struct{}{}
		edges = append(edges, graphEdge{From: from, To: to, Edge: label, Direction: dir})
	}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if _, seen := visited[item.id]; seen {
			continue
		}
		visited[item.id] = struct{}{}

		doc, ok := byID[item.id]
		if !ok {
			continue
		}
		nodes = append(nodes, doc)

		if item.depth >= maxDepth {
			continue
		}
		next := item.depth + 1

		// Outbound edges.
		for _, t := range doc.CTXSupersedes {
			addEdge(item.id, t, "supersedes", "outbound")
			queue = append(queue, qItem{t, next})
		}
		for _, t := range doc.ADRRelated {
			addEdge(item.id, t, "related", "outbound")
			queue = append(queue, qItem{t, next})
		}
		for _, t := range doc.TaskRequires {
			addEdge(item.id, t, "requires", "outbound")
			queue = append(queue, qItem{t, next})
		}
		for _, t := range doc.TaskDependsOn {
			addEdge(item.id, t, "depends_on", "outbound")
			queue = append(queue, qItem{t, next})
		}

		// Inbound (reverse) edges.
		for _, t := range doc.SupersededBy {
			addEdge(t, item.id, "supersedes", "inbound")
			queue = append(queue, qItem{t, next})
		}
		for _, ref := range doc.ReferencedBy {
			addEdge(ref.ID, item.id, ref.Edge, "inbound")
			queue = append(queue, qItem{ref.ID, next})
		}
		for _, t := range doc.ActiveTasks {
			addEdge(t, item.id, "active_tasks", "inbound")
			queue = append(queue, qItem{t, next})
		}
		for _, t := range doc.DerivedADRs {
			addEdge(t, item.id, "derived_adrs", "inbound")
			queue = append(queue, qItem{t, next})
		}
		for _, t := range doc.Blocks {
			addEdge(t, item.id, "blocks", "inbound")
			queue = append(queue, qItem{t, next})
		}
	}

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Edge < edges[j].Edge
	})

	return nodes, edges
}

// humanGraph renders a depth-first indented tree rooted at rootID.
// Already-visited nodes are printed inline with "(visited)" rather than
// being expanded again.
func humanGraph(rootID string, byID map[string]index.IndexedDoc, edges []graphEdge, maxDepth int) string {
	// Build per-node adjacency (both outbound and inbound from that node's POV).
	type nodeEdge struct {
		arrow  string // "→" or "←"
		label  string
		peerID string
	}
	adj := map[string][]nodeEdge{}
	for _, e := range edges {
		if e.Direction == "outbound" {
			adj[e.From] = append(adj[e.From], nodeEdge{"→", e.Edge, e.To})
		} else {
			adj[e.To] = append(adj[e.To], nodeEdge{"←", e.Edge, e.From})
		}
	}
	// Sort each adjacency list for determinism.
	for id := range adj {
		sort.Slice(adj[id], func(i, j int) bool {
			a, b := adj[id][i], adj[id][j]
			if a.arrow != b.arrow {
				return a.arrow < b.arrow
			}
			if a.label != b.label {
				return a.label < b.label
			}
			return a.peerID < b.peerID
		})
	}

	var sb strings.Builder
	visited := map[string]bool{}

	var render func(id string, depth int)
	render = func(id string, depth int) {
		indent := strings.Repeat("  ", depth)
		doc, ok := byID[id]
		if !ok {
			return
		}
		if depth == 0 {
			fmt.Fprintf(&sb, "%s%s — %s [%s%s]\n", indent, doc.ID, doc.CTXTitle, doc.Kind, docKindStatus(doc))
		}
		visited[id] = true
		if depth >= maxDepth {
			return
		}
		for _, ne := range adj[id] {
			peer, ok := byID[ne.peerID]
			if !ok {
				continue
			}
			extra := ""
			if visited[ne.peerID] {
				extra = " (visited)"
			}
			fmt.Fprintf(&sb, "%s  %s %-14s  %s — %s [%s%s]%s\n",
				indent, ne.arrow, ne.label, peer.ID, peer.CTXTitle, peer.Kind, docKindStatus(peer), extra)
			if !visited[ne.peerID] {
				render(ne.peerID, depth+1)
			}
		}
	}

	render(rootID, 0)
	return sb.String()
}

// docKindStatus returns ", <status>" for kinds that carry a status, else "".
func docKindStatus(doc index.IndexedDoc) string {
	s := docStatusField(doc)
	if s == "" {
		return ""
	}
	return ", " + s
}
