// Package index computes the enriched graph for a DocOps project and
// produces the docs/.index.json file described in ADR-0005.
//
// The core rule: every field written by humans lives in source frontmatter;
// every derived field lives here and is never hand-edited.
package index

// IndexVersion is incremented when the shape of IndexedDoc changes in a way
// that older readers cannot silently handle.
const IndexVersion = 1

// Index is the top-level .index.json structure.
type Index struct {
	GeneratedAt      string             `json:"generated_at"` // RFC3339
	Version          int                `json:"version"`
	Docs             []IndexedDoc       `json:"docs"`
	RecentAmendments []RecentAmendment  `json:"recent_amendments,omitempty"`
}

// RecentAmendment is a flattened amendment entry for the top-level
// `recent_amendments` list — one row per amendment within the activity
// window, sorted newest-first. Each row carries the ADR id so callers
// don't have to walk back into Docs.
type RecentAmendment struct {
	ADRID   string `json:"adr"`
	Date    string `json:"date"`
	Kind    string `json:"kind"`
	By      string `json:"by"`
	Summary string `json:"summary"`
	Ref     string `json:"ref,omitempty"`
}

// Reference is an inbound edge: some other doc pointed at this one via `edge`.
type Reference struct {
	ID   string `json:"id"`
	Edge string `json:"edge"`
}

// IndexedDoc is the union of all fields emitted for a single document.
// Fields only meaningful to one kind are omitted (zero values) in the JSON
// output via omitempty.
//
// Field ordering here is intentional: identity → source frontmatter → derived
// graph → computed status → metrics. encoding/json follows struct field order,
// giving us deterministic key ordering for free.
type IndexedDoc struct {
	// Identity (always present)
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Folder string `json:"folder"`
	Path   string `json:"path"`

	// Source frontmatter — Context
	CTXTitle      string   `json:"title,omitempty"`
	CTXType       string   `json:"type,omitempty"`
	CTXSupersedes []string `json:"supersedes,omitempty"`

	// Source frontmatter — ADR (title shared via CTXTitle above for brevity;
	// we use a single title field across all kinds)
	ADRStatus   string   `json:"status,omitempty"`
	ADRCoverage string   `json:"coverage,omitempty"`
	ADRDate     string   `json:"date,omitempty"`
	ADRRelated  []string `json:"related,omitempty"`
	ADRTags     []string `json:"tags,omitempty"`

	// Source frontmatter — Task
	TaskStatus    string   `json:"task_status,omitempty"`
	TaskPriority  string   `json:"priority,omitempty"`
	TaskAssignee  string   `json:"assignee,omitempty"`
	TaskRequires  []string `json:"requires,omitempty"`
	TaskDependsOn []string `json:"depends_on,omitempty"`

	// Derived text metrics
	Summary   string `json:"summary,omitempty"`
	WordCount int    `json:"word_count"`

	// Derived time fields
	LastTouched string `json:"last_touched"` // RFC3339; always present
	AgeDays     int    `json:"age_days"`

	// Reverse edges (all kinds)
	SupersededBy []string    `json:"superseded_by,omitempty"`
	ReferencedBy []Reference `json:"referenced_by,omitempty"`

	// Reverse edges — CTX only
	DerivedADRs []string `json:"derived_adrs,omitempty"`
	ActiveTasks []string `json:"active_tasks,omitempty"` // also ADR

	// Reverse edges — Task only
	Blocks []string `json:"blocks,omitempty"`

	// Computed fields — ADR only
	Implementation string             `json:"implementation,omitempty"`
	Amendments     []IndexedAmendment `json:"amendments,omitempty"`

	// Computed staleness — all kinds
	Stale bool `json:"stale"`
}

// IndexedAmendment mirrors schema.Amendment with JSON tags so callers
// (HTML viewer, CLI consumers) get a stable shape without depending on
// the schema package's YAML tags.
type IndexedAmendment struct {
	Date            string   `json:"date"`
	Kind            string   `json:"kind"`
	By              string   `json:"by"`
	Summary         string   `json:"summary"`
	AffectsSections []string `json:"affects_sections,omitempty"`
	Ref             string   `json:"ref,omitempty"`
}
