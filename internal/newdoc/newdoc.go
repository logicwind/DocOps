// Package newdoc implements `docops new <type>`: scaffold a new CTX, ADR, or
// Task document with valid frontmatter and an atomically allocated ID. The
// package is separated from cmd/docops so tests can drive it without spawning
// a subprocess.
package newdoc

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/loader"
	"github.com/logicwind/docops/internal/schema"
	"gopkg.in/yaml.v3"
)

// allocMu serializes ID allocation within a process. The advisory flock
// covers cross-process safety on unix; on Windows flock is a no-op, so
// this mutex is what prevents goroutines from racing in parallel tests
// and from the rare in-process concurrent CLI invocation.
var allocMu sync.Mutex

// DocType is the doc kind the caller wants to create.
type DocType string

const (
	DocTypeCtx  DocType = "ctx"
	DocTypeADR  DocType = "adr"
	DocTypeTask DocType = "task"
)

// Options configures a single doc-creation run.
type Options struct {
	// Root is the project root (where docops.yaml lives).
	Root string

	// Type selects the doc kind.
	Type DocType

	// Title is the human-readable document title.
	Title string

	// CtxType is the `type:` field value for CTX docs (default: "memo").
	CtxType string

	// Related is a comma-separated list of IDs for ADR docs.
	Related []string

	// Requires is the list of ADR/CTX IDs for Task docs.
	Requires []string

	// Priority is the task priority (default: "p2").
	Priority string

	// Assignee is the task assignee (default: "unassigned").
	Assignee string

	// NoOpen skips launching $EDITOR after creation.
	NoOpen bool

	// JSON causes Run to emit {"id","path"} to stdout instead of the human message.
	JSON bool

	// cfg is loaded internally; exposed only for seeding tests.
	cfg config.Config
}

// Result is returned by Run on success.
type Result struct {
	ID   string
	Path string // absolute path
	Rel  string // relative to project root
}

// Run creates the new document and returns its ID and path.
func Run(opts Options) (*Result, error) {
	if opts.Root == "" {
		return nil, fmt.Errorf("newdoc: Root must be set")
	}
	abs, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("newdoc: resolve root: %w", err)
	}
	opts.Root = abs

	cfg, _, err := config.FindAndLoad(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("newdoc: load config: %w", err)
	}
	opts.cfg = cfg

	if err := validate(opts); err != nil {
		return nil, err
	}

	id, err := allocateID(opts)
	if err != nil {
		return nil, err
	}

	slug := slugify(opts.Title)
	rel, absPath := docPath(opts, id, slug)

	body := buildDoc(opts, id)

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return nil, fmt.Errorf("newdoc: mkdir: %w", err)
	}
	if err := os.WriteFile(absPath, body, 0o644); err != nil {
		return nil, fmt.Errorf("newdoc: write: %w", err)
	}

	res := &Result{ID: id, Path: absPath, Rel: rel}

	if !opts.NoOpen && !opts.JSON && shouldOpenEditor() {
		openEditor(absPath)
	}

	return res, nil
}

// validate checks required fields before any allocation happens.
func validate(opts Options) error {
	switch opts.Type {
	case DocTypeCtx, DocTypeADR, DocTypeTask:
	default:
		return fmt.Errorf("newdoc: unknown type %q — must be ctx, adr, or task", opts.Type)
	}
	if strings.TrimSpace(opts.Title) == "" {
		return fmt.Errorf("newdoc: title must not be empty")
	}
	if opts.Type == DocTypeTask && len(opts.Requires) == 0 {
		return fmt.Errorf("newdoc: tasks must cite at least one ADR or CTX via --requires (ADR-0004)")
	}
	return nil
}

// docPath returns (relative, absolute) paths for the new document.
func docPath(opts Options, id, slug string) (string, string) {
	var dir, filename string
	switch opts.Type {
	case DocTypeCtx:
		dir = opts.cfg.Paths.Context
		filename = id + "-" + slug + ".md"
	case DocTypeADR:
		dir = opts.cfg.Paths.Decisions
		filename = id + "-" + slug + ".md"
	case DocTypeTask:
		dir = opts.cfg.Paths.Tasks
		filename = id + "-" + slug + ".md"
	}
	rel := filepath.Join(dir, filename)
	return rel, filepath.Join(opts.Root, rel)
}

// buildDoc constructs the complete markdown document bytes.
func buildDoc(opts Options, id string) []byte {
	var fm []byte
	var bodySection string

	switch opts.Type {
	case DocTypeCtx:
		ctxType := opts.CtxType
		if ctxType == "" {
			ctxType = "memo"
		}
		c := schema.Context{
			Title:      opts.Title,
			Type:       ctxType,
			Supersedes: []string{},
		}
		fm = marshalFrontmatter(c)
		bodySection = fmt.Sprintf("# %s\n\n<!-- Add content here -->\n", opts.Title)

	case DocTypeADR:
		related := opts.Related
		if related == nil {
			related = []string{}
		}
		a := schema.ADR{
			Title:      opts.Title,
			Status:     "draft",
			Coverage:   "required",
			Date:       time.Now().Format("2006-01-02"),
			Supersedes: []string{},
			Related:    related,
			Tags:       []string{},
		}
		fm = marshalFrontmatter(a)
		bodySection = fmt.Sprintf("# %s\n\n<!-- Add content here -->\n", opts.Title)

	case DocTypeTask:
		priority := opts.Priority
		if priority == "" {
			priority = "p2"
		}
		assignee := opts.Assignee
		if assignee == "" {
			assignee = "unassigned"
		}
		t := schema.Task{
			Title:     opts.Title,
			Status:    "backlog",
			Priority:  priority,
			Assignee:  assignee,
			Requires:  opts.Requires,
			DependsOn: []string{},
		}
		fm = marshalFrontmatter(t)
		bodySection = fmt.Sprintf("# %s\n\n## Goal\n\n## Acceptance\n\n## Notes\n", opts.Title)
	}

	return []byte("---\n" + string(fm) + "---\n\n" + bodySection)
}

// marshalFrontmatter serialises a struct to YAML. Uses yaml.v3 so field order
// matches the struct tag order, which matches the existing doc style.
func marshalFrontmatter(v any) []byte {
	// Marshal to node tree first so we can emit flow-style sequences that
	// match the existing doc style (e.g. `requires: [ADR-0004]`).
	var node yaml.Node
	if err := node.Encode(v); err != nil {
		panic("newdoc: yaml encode: " + err.Error())
	}
	setFlowSeq(&node)

	out, err := yaml.Marshal(&node)
	if err != nil {
		panic("newdoc: yaml marshal: " + err.Error())
	}
	return out
}

// setFlowSeq walks a yaml.Node tree and sets all sequence nodes to flow style
// so arrays are emitted as `[a, b]` rather than block-style lists.
func setFlowSeq(n *yaml.Node) {
	if n.Kind == yaml.SequenceNode {
		n.Style = yaml.FlowStyle
	}
	for _, child := range n.Content {
		setFlowSeq(child)
	}
}

// slugMaxLen is the maximum byte length of the slug portion of a filename.
const slugMaxLen = 50

// nonAlnum matches anything that is not a letter or digit.
var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// slugify converts a title to a lowercase, hyphen-separated, ASCII slug
// truncated at slugMaxLen characters.
func slugify(title string) string {
	// Fold to lowercase ASCII: drop non-ASCII, keep letters+digits.
	var buf strings.Builder
	for _, r := range title {
		if r > unicode.MaxASCII {
			continue
		}
		buf.WriteRune(unicode.ToLower(r))
	}
	s := nonAlnum.ReplaceAllString(buf.String(), "-")
	s = strings.Trim(s, "-")
	if len(s) > slugMaxLen {
		s = s[:slugMaxLen]
		// Trim a trailing partial word separator.
		s = strings.TrimRight(s, "-")
	}
	return s
}

// shouldOpenEditor returns true when it is appropriate to launch an editor.
// We skip when running in CI or when $EDITOR is unset.
func shouldOpenEditor() bool {
	if os.Getenv("CI") != "" {
		return false
	}
	return os.Getenv("EDITOR") != ""
}

// defaultEditor picks a platform-appropriate fallback.
func defaultEditor() string {
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	return "vim"
}

// openEditor launches $EDITOR (or the platform default) on path. Errors are
// silently swallowed — the file was already written successfully.
func openEditor(path string) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = defaultEditor()
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

// --- ID allocation ----------------------------------------------------------

// counters is the on-disk counter file format.
type counters struct {
	Version int            `json:"version"`
	Next    map[string]int `json:"next"`
}

// allocateID claims the next ID for the chosen kind using OS file locking
// across processes (unix flock; no-op on Windows) plus a process-level
// mutex that covers goroutines within the same binary — the flock alone
// cannot serialize threads of one process on all platforms.
func allocateID(opts Options) (string, error) {
	allocMu.Lock()
	defer allocMu.Unlock()

	counterPath := filepath.Join(opts.Root, opts.cfg.Paths.Counters)
	lockPath := counterPath + ".lock"

	if err := os.MkdirAll(filepath.Dir(counterPath), 0o755); err != nil {
		return "", fmt.Errorf("newdoc: counters dir: %w", err)
	}

	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return "", fmt.Errorf("newdoc: open lock: %w", err)
	}
	defer lf.Close()

	if err := flock(lf); err != nil {
		return "", fmt.Errorf("newdoc: acquire lock: %w", err)
	}
	defer funlock(lf) //nolint:errcheck

	// Read or seed the counter file inside the lock.
	c, err := readCounters(counterPath)
	if err != nil {
		// Seed from existing documents.
		c, err = seedCounters(opts.Root, opts.cfg)
		if err != nil {
			return "", fmt.Errorf("newdoc: seed counters: %w", err)
		}
	}

	prefix := kindPrefix(opts.Type)
	n := c.Next[prefix]
	c.Next[prefix] = n + 1

	if err := writeCounters(counterPath, c); err != nil {
		return "", fmt.Errorf("newdoc: write counters: %w", err)
	}

	return formatID(opts.Type, n), nil
}

// kindPrefix maps a DocType to its counter map key.
func kindPrefix(t DocType) string {
	switch t {
	case DocTypeCtx:
		return "CTX"
	case DocTypeADR:
		return "ADR"
	case DocTypeTask:
		return "TP"
	}
	return ""
}

// formatID formats an ID with the correct zero-padding per ADR-0003:
// ADRs are 4-digit; CTX and TP are 3-digit.
func formatID(t DocType, n int) string {
	switch t {
	case DocTypeADR:
		return fmt.Sprintf("ADR-%04d", n)
	case DocTypeCtx:
		return fmt.Sprintf("CTX-%03d", n)
	case DocTypeTask:
		return fmt.Sprintf("TP-%03d", n)
	}
	return ""
}

// readCounters reads and parses the counter file. Returns an error if absent.
func readCounters(path string) (counters, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return counters{}, err
	}
	var c counters
	if err := json.Unmarshal(raw, &c); err != nil {
		return counters{}, fmt.Errorf("parse counters: %w", err)
	}
	if c.Next == nil {
		c.Next = map[string]int{}
	}
	return c, nil
}

// writeCounters writes atomically (temp file + rename).
func writeCounters(path string, c counters) error {
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// seedCounters scans existing IDs using the loader and returns a fresh counter
// struct seeded to max-existing+1 per kind.
func seedCounters(root string, cfg config.Config) (counters, error) {
	set, err := loader.Load(root, cfg)
	if err != nil {
		// If docs/ doesn't exist yet, start from 1 for all kinds.
		return counters{Version: 1, Next: map[string]int{"CTX": 1, "ADR": 1, "TP": 1}}, nil
	}

	maxes := map[string]int{"CTX": 0, "ADR": 0, "TP": 0}
	for id := range set.Docs {
		prefix, numStr, ok := splitID(id)
		if !ok {
			continue
		}
		n, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		if n > maxes[prefix] {
			maxes[prefix] = n
		}
	}

	next := map[string]int{
		"CTX": maxes["CTX"] + 1,
		"ADR": maxes["ADR"] + 1,
		"TP":  maxes["TP"] + 1,
	}
	return counters{Version: 1, Next: next}, nil
}

// splitID breaks "ADR-0012" into ("ADR", "0012", true).
func splitID(id string) (prefix, num string, ok bool) {
	idx := strings.LastIndex(id, "-")
	if idx < 0 {
		return "", "", false
	}
	return id[:idx], id[idx+1:], true
}
