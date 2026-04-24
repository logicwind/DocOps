// Package htmlviewer ships a single-page HTML viewer for a DocOps
// repository. Both `docops html` (static emitter) and `docops serve`
// (localhost server) consume the same embedded SPA and the same
// in-memory index produced by internal/index.
//
// Design: Go does almost nothing. It emits the SPA bytes, the
// serialized index, the STATE.md, and copies of each source markdown
// body. The browser renders sidebar, detail pane, frontmatter, and
// the cytoscape graph entirely client-side. Libraries come from
// jsDelivr at first load. See ADR-0027.
package htmlviewer

import _ "embed"

// SPA is the single-file viewer. Served verbatim at `/index.html` and
// copied into `--output` by `docops html`.
//
//go:embed index.html
var SPA []byte
