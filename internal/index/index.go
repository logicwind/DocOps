package index

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nachiket/docops/internal/config"
	"github.com/nachiket/docops/internal/loader"
	"github.com/nachiket/docops/internal/schema"
)

// ctxRefRE matches a CTX-NNN reference anywhere in a body.
var ctxRefRE = regexp.MustCompile(`\bCTX-\d+\b`)

// refEntry is an inbound edge used during index construction before it is
// converted to the exported Reference type.
type refEntry struct{ id, edge string }

// Build computes the enriched index for the given DocSet.
// now is the reference timestamp used for age_days and stale calculations;
// pass time.Now() for production use; inject a fixed time in tests.
func Build(set *loader.DocSet, cfg config.Config, root string, now time.Time) (*Index, error) {
	// Read all body bytes once so we can build summaries and body-scan for
	// CTX references without re-opening files.
	bodies := make(map[string][]byte, len(set.Docs))
	for id, doc := range set.Docs {
		raw, err := os.ReadFile(filepath.Join(root, doc.Path))
		if err != nil {
			return nil, err
		}
		_, body, err := schema.SplitFrontmatter(raw)
		if err != nil {
			// If frontmatter is broken the validator should have caught it;
			// treat remaining bytes as the body so we still produce an entry.
			body = raw
		}
		bodies[id] = body
	}

	// Pre-compute reverse edge maps in a single pass over all docs.
	supersededBy := map[string][]string{}                // target → []source IDs
	referencedBy := map[string][]refEntry{}              // target → [{id,edge}]
	activeTasks  := map[string][]string{}                // adr/ctx ID → []task IDs with status active
	blocks       := map[string][]string{}                // task ID → []task IDs that depend on it
	derivedADRs  := map[string]map[string]struct{}{}     // ctx ID → set of ADR IDs

	addRef := func(target, sourceID, edge string) {
		referencedBy[target] = append(referencedBy[target], refEntry{sourceID, edge})
	}

	for _, id := range set.Order {
		doc := set.Docs[id]

		// supersedes (ADR→ADR, CTX→CTX)
		for _, target := range doc.Supersedes() {
			supersededBy[target] = append(supersededBy[target], id)
			addRef(target, id, "supersedes")
		}

		// related (ADR→ADR, ADR→CTX)
		for _, target := range doc.Related() {
			addRef(target, id, "related")
		}

		// requires (Task→ADR, Task→CTX)
		for _, target := range doc.Requires() {
			addRef(target, id, "requires")
			if doc.Kind == schema.KindTask && doc.Status() == "active" {
				activeTasks[target] = append(activeTasks[target], id)
			}
		}

		// depends_on (Task→Task)
		for _, target := range doc.DependsOn() {
			blocks[target] = append(blocks[target], id)
			addRef(target, id, "depends_on")
		}

		// derived_adrs: ADRs whose related array or body reference a CTX ID.
		if doc.Kind == schema.KindADR {
			// Via related array
			for _, target := range doc.Related() {
				if strings.HasPrefix(target, "CTX-") {
					if derivedADRs[target] == nil {
						derivedADRs[target] = map[string]struct{}{}
					}
					derivedADRs[target][id] = struct{}{}
				}
			}
			// Via body text scan
			for _, match := range ctxRefRE.FindAllString(string(bodies[id]), -1) {
				if derivedADRs[match] == nil {
					derivedADRs[match] = map[string]struct{}{}
				}
				derivedADRs[match][id] = struct{}{}
			}
		}
	}

	// Sort every reverse-edge slice for determinism.
	for k := range supersededBy {
		sort.Strings(supersededBy[k])
	}
	for k := range activeTasks {
		sort.Strings(activeTasks[k])
	}
	for k := range blocks {
		sort.Strings(blocks[k])
	}
	for k := range referencedBy {
		sort.Slice(referencedBy[k], func(i, j int) bool {
			if referencedBy[k][i].id != referencedBy[k][j].id {
				return referencedBy[k][i].id < referencedBy[k][j].id
			}
			return referencedBy[k][i].edge < referencedBy[k][j].edge
		})
	}

	// Build sorted IDs for output determinism.
	sortedIDs := make([]string, len(set.Order))
	copy(sortedIDs, set.Order)
	sort.Strings(sortedIDs)

	docs := make([]IndexedDoc, 0, len(sortedIDs))
	for _, id := range sortedIDs {
		doc := set.Docs[id]
		body := bodies[id]

		summary, wordCount := bodySummary(body)
		lastTouched, _ := gitLastTouched(root, doc.Path)
		ageDays := int(now.Sub(lastTouched).Hours() / 24)
		if ageDays < 0 {
			ageDays = 0
		}

		// Collect citing tasks for ADR implementation derivation.
		citingTasks := citingTasksFor(id, set)

		idoc := IndexedDoc{
			ID:          id,
			Kind:        string(doc.Kind),
			Folder:      filepath.Dir(doc.Path),
			Path:        doc.Path,
			Summary:     summary,
			WordCount:   wordCount,
			LastTouched: lastTouched.UTC().Format(time.RFC3339),
			AgeDays:     ageDays,
		}

		// Source frontmatter fields per kind.
		switch doc.Kind {
		case schema.KindContext:
			if doc.Context != nil {
				idoc.CTXTitle = doc.Context.Title
				idoc.CTXType = doc.Context.Type
				if len(doc.Context.Supersedes) > 0 {
					idoc.CTXSupersedes = doc.Context.Supersedes
				}
			}

		case schema.KindADR:
			if doc.ADR != nil {
				idoc.CTXTitle = doc.ADR.Title // shared title field
				idoc.ADRStatus = doc.ADR.Status
				idoc.ADRCoverage = doc.ADR.Coverage
				idoc.ADRDate = doc.ADR.Date
				if len(doc.ADR.Supersedes) > 0 {
					idoc.CTXSupersedes = doc.ADR.Supersedes
				}
				if len(doc.ADR.Related) > 0 {
					idoc.ADRRelated = doc.ADR.Related
				}
				if len(doc.ADR.Tags) > 0 {
					idoc.ADRTags = doc.ADR.Tags
				}
			}

		case schema.KindTask:
			if doc.Task != nil {
				idoc.CTXTitle = doc.Task.Title // shared title field
				idoc.TaskStatus = doc.Task.Status
				idoc.TaskPriority = doc.Task.Priority
				idoc.TaskAssignee = doc.Task.Assignee
				if len(doc.Task.Requires) > 0 {
					idoc.TaskRequires = doc.Task.Requires
				}
				if len(doc.Task.DependsOn) > 0 {
					idoc.TaskDependsOn = doc.Task.DependsOn
				}
			}
		}

		// Reverse edges.
		if sb := supersededBy[id]; len(sb) > 0 {
			idoc.SupersededBy = sb
		}
		if rb := referencedBy[id]; len(rb) > 0 {
			refs := make([]Reference, len(rb))
			for i, r := range rb {
				refs[i] = Reference{ID: r.id, Edge: r.edge}
			}
			idoc.ReferencedBy = refs
		}

		switch doc.Kind {
		case schema.KindContext:
			// derived_adrs: sorted slice from the map.
			if m := derivedADRs[id]; len(m) > 0 {
				sl := sortedKeys(m)
				idoc.DerivedADRs = sl
			}
			if at := activeTasks[id]; len(at) > 0 {
				idoc.ActiveTasks = at
			}

		case schema.KindADR:
			if at := activeTasks[id]; len(at) > 0 {
				idoc.ActiveTasks = at
			}
			idoc.Implementation = computeImplementation(doc, citingTasks)

		case schema.KindTask:
			if bl := blocks[id]; len(bl) > 0 {
				idoc.Blocks = bl
			}
		}

		// Stale flag.
		idoc.Stale = computeStale(doc, idoc, citingTasks, derivedADRs, referencedBy, cfg, ageDays)

		docs = append(docs, idoc)
	}

	return &Index{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Version:     IndexVersion,
		Docs:        docs,
	}, nil
}

// citingTasksFor returns all Task docs whose requires list contains targetID.
func citingTasksFor(targetID string, set *loader.DocSet) []*loader.Doc {
	var out []*loader.Doc
	for _, id := range set.Order {
		doc := set.Docs[id]
		if doc.Kind != schema.KindTask {
			continue
		}
		for _, req := range doc.Requires() {
			if req == targetID {
				out = append(out, doc)
				break
			}
		}
	}
	return out
}

// computeImplementation derives the implementation field for an ADR per
// ADR-0010. citingTasks must be the tasks whose requires contains this ADR's
// ID.
func computeImplementation(doc *loader.Doc, citingTasks []*loader.Doc) string {
	if doc.ADR == nil {
		return ""
	}
	if doc.ADR.Coverage == "not-needed" {
		return "n/a"
	}
	if len(citingTasks) == 0 {
		return "not-started"
	}

	var nDone, nActive, nOther int
	for _, t := range citingTasks {
		switch t.Status() {
		case "done":
			nDone++
		case "active":
			nActive++
		default:
			nOther++ // backlog, blocked
		}
	}

	switch {
	case nActive > 0:
		return "in-progress"
	case nDone > 0 && nOther == 0:
		return "done"
	case nDone > 0 && nOther > 0:
		return "partial"
	default:
		// All tasks are backlog/blocked, none active, none done.
		return "not-started"
	}
}

// computeStale derives the stale boolean per kind using the gap thresholds in
// cfg.Gaps.
func computeStale(
	doc *loader.Doc,
	idoc IndexedDoc,
	citingTasks []*loader.Doc,
	derivedADRs map[string]map[string]struct{},
	referencedBy map[string][]refEntry,
	cfg config.Config,
	ageDays int,
) bool {
	switch doc.Kind {
	case schema.KindADR:
		if doc.ADR == nil {
			return false
		}
		if doc.ADR.Status == "accepted" &&
			doc.ADR.Coverage == "required" &&
			len(citingTasks) == 0 &&
			ageDays > cfg.Gaps.AdrAcceptedNoTasksAfterDays {
			return true
		}
		if doc.ADR.Status == "draft" &&
			ageDays > cfg.Gaps.AdrDraftStaleDays {
			return true
		}
		return false

	case schema.KindTask:
		if doc.Task == nil {
			return false
		}
		return doc.Task.Status == "active" && ageDays > cfg.Gaps.TaskActiveNoCommitsDays

	case schema.KindContext:
		hasDerivedADRs := len(derivedADRs[doc.ID]) > 0
		hasReferencedBy := len(referencedBy[doc.ID]) > 0
		return ageDays > cfg.Gaps.CtxWithNoDerivedLinksAfterDays &&
			!hasDerivedADRs && !hasReferencedBy
	}
	return false
}

// sortedKeys returns the keys of a map[string]struct{} in sorted order.
func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
