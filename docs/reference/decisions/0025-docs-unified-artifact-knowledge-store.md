---
id: "0025"
title: "Docs — unified artifact + knowledge store, provenance-tagged, plugin-composed"
date: 2026-04-18
status: proposed
implementation: not-started
supersedes: ["0011"]
superseded_by: null
related: ["0015", "0016", "0020", "0021", "0023", "0024"]
tags: [docs, knowledge, artifacts, plugins, rag, vector-search, l2, l3]
---

# 0025 · Docs — unified artifact + knowledge store, provenance-tagged, plugin-composed

Date: 2026-04-18
Status: proposed
Supersedes: ADR 0011 (artifact storage — subsumed)
Related: ADR 0015 (L2 pluggability), ADR 0016 (memory pluggability
pattern), ADR 0020 (plugin architecture — adds new service kinds),
ADR 0021 (console Work tab — UI placement), ADR 0023 (dispatcher
treats `DocsSearch` as capability), ADR 0024 (apps vs packages layout)

## Context

Zund needs two things that look different at first and identical once
you squint:

1. **Artifacts** — agent-generated outputs the user reads, downloads, or
   hands back to a later agent. ADR 0011 covered this: explicit emission,
   content-addressed, pluggable backend, under `~/.zund/data/artifacts/`.
2. **Knowledge** — user-provided reference material (brand guides, PDFs,
   analytics exports, wiki pages) that agents retrieve from during work.
   `roadmap/next.md` item E flagged this as open: same engine as memory
   or separate, scope (fleet/team/agent), ingest pipeline.

Building them as two stores causes three problems:

- **Explicit handoff between agents.** Researcher writes an artifact;
  blog-writer needs it as knowledge. With two stores that's an API —
  "promote artifact to knowledge" — with its own permissions, index
  refresh, and race conditions.
- **Duplicate infrastructure.** Chunking, embedding, vector index, file
  parsing, metadata schema, and UI are needed in both. Two of each is
  waste.
- **User confusion.** Users don't think about whether a PDF they uploaded
  "is knowledge" vs "is an artifact" — they think "this is a file I want
  my agents to see." The distinction is *provenance*, not *kind*.

The right primitive is **one shared docs store** with provenance as a
flag, not a storage boundary.

## Decision

Zund consolidates artifacts and knowledge into a single **Docs** service:
one content-addressed backend, one metadata schema, one vector index,
one UI. Provenance, trust, and indexing are per-document flags. The
`artifacts` plugin kind from ADR 0020 is replaced by `docs` plus four
supporting kinds (`parser`, `chunker`, `embedder`, `vector-index`).

### Data model

Every doc has:

```
id           : ULID (stable, not content hash — filename may change)
path         : string             # e.g. "projects/q2/brand-guide.pdf"
content_sha  : hex(sha256(bytes)) # dedup key at the blob layer
source       : "user" | "agent"   # immutable, set at creation
author       : string             # user id or "agent:<name>"
knowledge    : boolean            # user-toggled "canonical reference" flag
indexed      : boolean            # in vector index (default true)
writable_by_agents : boolean      # default true for agent docs, false for user docs
mime         : string
size         : bytes
created_at   : iso8601
updated_at   : iso8601

# metadata (three layers)
tags         : Record<string, string>   # user/agent-set labels: client=acme, project=q2, reviewed_by=nix
custom       : Record<string, unknown>  # arbitrary structured metadata (extracted / computed)
structure    : {                        # parser-emitted, optional — populated by whichever parser ran
  outline    : HeadingNode[]            # hierarchical heading tree
  tables     : StructuredTable[]        # structured table data (when parser supports it)
  figures    : Figure[]                 # images / diagrams with captions
  frontmatter: Record<string, unknown>  # YAML/TOML frontmatter auto-extracted from md
  links      : Link[]                   # internal refs + external URLs
  pages      : number                   # page count for paginated sources
  language   : string                   # detected (ISO 639-1)
}
```

Two axes, one truth: `source` is **who authored** (immutable); `knowledge`
is **user-curated trust** (mutable). They're orthogonal — user uploads
can be flagged `knowledge: false` (a scratch note) and agent outputs can
be promoted to `knowledge: true` after review.

**Metadata propagation.** Markdown frontmatter auto-populates `tags`
and `custom` on ingest (YAML `tags: [a, b]` lands in `tags`; the rest
in `custom.frontmatter`). Parsers populate `structure` at their
discretion — LiteParse fills `outline` (visually inferred), `figures`,
and `pages`; Docling fills everything including structured `tables`;
`parser-vision-llm` fills whatever the prompt asks for. Consumers never
assume a field is populated — always treat `structure.*` as optional.

### Chunk metadata

Every chunk produced by a parser+chunker carries a metadata envelope
that rides with it through the embedder into the vector index payload:

```
chunk: {
  doc_id       : ULID               # parent doc
  chunk_idx    : number             # stable ordering within doc
  text         : string             # the chunk content
  page         : number?            # for paginated sources (PDF, DOCX with pages)
  section_path : string[]?          # ["Introduction", "Methods", "Dataset"]
  bbox         : [x,y,w,h]?         # for visual sources (LiteParse emits these)
  heading      : string?            # nearest heading for the chunk
  token_count  : number             # approximate
  source       : "user" | "agent"   # inherited from doc, denormalized for filter speed
  knowledge    : boolean            # inherited for same reason
  tags         : Record<string,string>  # inherited for same reason
}
```

Three reasons this matters:

1. **Citations.** Agents quoting retrievals can include "(Brand Guide,
   p.14, §Voice)" — far more trustworthy than bare text chunks.
2. **Retrieval filters.** `DocsSearch.query(q, filter: { tags: { client:
   "acme" }, page: { gte: 30 } })` — narrow the candidate set before
   vector ranking. Denormalizing `source`/`knowledge`/`tags` onto the
   chunk avoids a join on every filter.
3. **UI deep-linking.** Click a search result, open the doc at that
   page with the chunk highlighted. Requires `page` + `bbox`.

### Reserved paths

The system reserves only the paths it *must* own:

- `agents/<agent-name>/` — default write root for a given agent. An
  agent can write here without config. Writing elsewhere requires an
  explicit `output_paths:` declaration in the agent manifest.
- `inbox/` — user drop zone. Flat root = "any agent may pick up."
  `inbox/<agent-name>/` = targeted to a specific agent.

Everything else is **user-owned free-form tree**. Users organize as they
like (`projects/acme/`, `research/papers/`, `brand/`). The system does
not impose a `knowledge/` root — that would force a hierarchy on the
user. Knowledge is a flag, not a folder.

**No system-driven moves.** Flipping `knowledge: true` leaves the file
where it is. Users can move files manually (drag in UI or `zund docs mv`);
paths and IDs stay stable under rename only.

### Inbox, not inbox+outbox

There is no `outbox/`. Agent outputs live under `agents/<name>/` — that
is the outbox, structured by author. The asymmetry is deliberate: inbox
is a shared queue with unread semantics; outputs are per-agent
workspaces. Using a single symmetric metaphor would misrepresent both.

### Plugin decomposition

Following ADR 0020 (each plugin is independent, owns its own storage),
Docs is composed of five service kinds:

```
kind: docs           ← new (replaces artifacts from ADR 0020)
  contract: DocsStore, DocsSearch
  default:  docs-local        (filesystem + SQLite metadata)
  options:  docs-s3, docs-minio, docs-gcs, docs-azure-blob

kind: parser         ← new
  contract: Parser  (bytes + mime → text chunks + structure)
  default:  parser-native        (md, txt, html, json, csv, code — fallback)
  recommended (local):  parser-liteparse   (TS-native, Bun-fit)
  vision-llm alternative:  parser-vision-llm   (provider-agnostic VLM)
  options:  parser-docling, parser-marker, parser-markitdown, parser-mineru,
            parser-mistral-ocr, parser-llamaparse, parser-unstructured,
            parser-reducto, parser-azure-di, parser-aws-textract, parser-google-docai

kind: chunker        ← new
  contract: Chunker (text → spans with metadata)
  default:  chunker-recursive   (hand-rolled recursive text splitter)
  options:  chunker-semantic, chunker-markdown-aware,
            chunker-code-aware, chunker-llamaindex

kind: embedder       ← new
  contract: Embedder (text[] → vector[])
  default:  embedder-ollama     (nomic-embed-text, 768-dim, local)
  options:  embedder-voyage, embedder-openai, embedder-cohere,
            embedder-mistral, embedder-bge-local

kind: vector-index   ← new
  contract: VectorIndex (upsert, query, delete by id)
  default:  vector-sqlite-vec   (in-process, zero new infra)
  options:  vector-qdrant, vector-lancedb, vector-pgvector,
            vector-milvus, vector-weaviate
```

The `docs` plugin imports the other four by contract — it does not know
which implementation is wired behind them. Swap the embedder (e.g.
Ollama → Voyage) by installing a different plugin; `docs` is unchanged.

Other plugins (`agents`, `chat`, `dispatcher`) import `DocsStore` and
`DocsSearch`. Inter-plugin composition is the point of the architecture,
not an accident of it. The researcher → blog-writer flow is just:
researcher writes to `agents/researcher/notes.md`, blog-writer calls
`DocsSearch.query("brand guidelines Q2")` and the retrieval returns
chunks from both the researcher's notes and user-uploaded brand guides,
tagged with provenance so the consumer can weight accordingly.

### Storage backend: local default, production options

**Default: `docs-local`** — filesystem + SQLite.

```
~/.zund/plugins/docs-local/
  blobs/<sha[0:2]>/<sha>         # content-addressed blob pool (dedup)
  tree/<path>                    # symlink or index mapping path → blob
  meta.db                        # SQLite: docs, chunks, embeddings (sqlite-vec)
```

Content-addressed dedup is preserved from ADR 0011. Path→blob mapping
lives in SQLite so rename/move is metadata-only (no bytes copied).

**Production: `docs-s3` / `docs-minio`** — same layout, S3/MinIO for
blobs, managed Postgres (or RDS, Cloud SQL) for metadata + pgvector, or
external vector DB behind the `vector-index` contract. This is the seam
OSS→Pro runs through (ADR 0020 OSS-boundary concern): the same
interface, a different bundle of plugins.

MinIO is an explicit first-class option for self-hosters who want
S3-compatible storage without a cloud account. Drops in behind the same
`DocsStore` contract.

### Chunking strategy

Chunking is its own plugin kind because the right chunker depends on the
content:

- **Recursive text splitter** (default) — hand-rolled, ~50 LoC, respects
  paragraph and sentence boundaries with overlap. Good enough for most
  prose.
- **Markdown-aware** — splits by heading hierarchy, preserves section
  context.
- **Code-aware** — AST-based splits by function/class boundaries (uses
  tree-sitter; per-language plugins).
- **Semantic / LLM-driven** — groups by topic using an embedding
  similarity threshold; higher quality, higher cost.

Per-doc override: `chunker: markdown-aware` in frontmatter or ingest
command. Defaults per MIME type are defined in the `docs` plugin config.

### Parser selection (local + online)

**Parsers are plugins** because PDF/DOCX/XLSX/HTML conversion is the
single hardest quality dial in a RAG system, and opinions here change
monthly. Shipping one hardcoded parser locks users out of whatever
is SOTA next quarter.

**Bundled native (`parser-native`):** md, txt, html, json, csv, code
files. No external deps. Fallback for formats no other parser claims.

**Recommended default local plugin:**

- **`parser-liteparse`** — LlamaIndex's LiteParse (released March 2026).
  **TypeScript-native**, which makes it the natural fit for zund's Bun +
  TS daemon — no Python subprocess, no venv, no ML stack. Zero Python
  deps, no GPU, ~500 pages / 2s on commodity CPU. Uses PDF.js for
  selectable-text PDFs and projects extracted text onto a spatial grid
  (preserves layout via whitespace + bounding boxes rather than lossy
  markdown conversion). Office docs via LibreOffice, images via
  ImageMagick + OCR. Emits page screenshots alongside text for
  multimodal agents. Trade-off: output is layout-faithful spatial text,
  not semantic markdown — headings and tables come out as *positioned*
  text, not as `#`/`##` or markdown tables. For RAG retrieval this is
  fine; if users want human-readable markdown or structured table
  extraction, point them to `parser-docling` or `parser-marker`.
  **Recommended default once available on npm as a Bun-compatible
  package.**

**Other local plugins (contrib, installable when users want specific
strengths):**

- **`parser-docling`** — IBM Research's Docling. Best-in-class for
  *structured* RAG: layout analysis via DocLayNet, table extraction via
  TableFormer, multilingual OCR (Tesseract/EasyOCR/RapidOCR). Preserves
  semantic hierarchy as a structured `DoclingDocument`. Python runtime
  (daemon bridges via subprocess or sidecar). Pick this when semantic
  markdown output and structured tables matter more than runtime
  simplicity.
- **`parser-marker`** — PDF/DOCX/PPTX/XLSX/HTML/EPUB → Markdown/JSON.
  Uses Surya OCR. Strong general-purpose option; runs on CPU/GPU/MPS.
  Python runtime. Good if semantic markdown output is required but
  Docling is too heavy.
- **`parser-markitdown`** — Microsoft's file-to-Markdown. Broad format
  coverage (PDF, Office, images, audio transcripts). Fast but weaker on
  complex tables and layout. Python runtime. Good for high-volume,
  low-complexity ingest where layout fidelity doesn't matter.
- **`parser-mineru`** — Best-in-class for CJK (Chinese/Japanese/Korean)
  and academic-paper layouts. Install only when you need it.
- **`parser-unstructured`** — Broad format coverage, production-hardened,
  active community. Heavier dep surface than Marker.

**Vision-LLM parser (provider-agnostic):**

- **`parser-vision-llm`** — feeds page screenshots (or raw images) to a
  vision-capable LLM and parses the model's response into structured
  text/markdown/JSON. Provider-agnostic: the plugin configures the
  backing model — local via Ollama (Gemma 3/4-class VLMs, qwen2-vl,
  llava, minicpm-v), or hosted (Gemini, Claude vision, GPT-4V, Mistral
  Pixtral). Screenshots come from whatever upstream parser produced
  them — LiteParse emits them natively for PDFs/images; for pure-image
  inputs (`image/png`, scanned PDFs) the VLM is the primary path.
  Useful when layout is complex enough that spatial text alone is
  insufficient (scanned docs, handwriting, mixed diagrams + text,
  hand-drawn content) and the user accepts the cost/latency/hallucination
  trade-off. Run fully local via Ollama for air-gapped setups; swap to
  a cloud provider for accuracy at the expense of privacy. Never set
  this as the bulk-ingest default — reserve for docs where the
  default parser's output is visibly wrong.

**Recommended online plugins (API-backed, bring-your-own-key):**

- **`parser-mistral-ocr`** — Mistral OCR 3 (released late 2025). SOTA
  layout and table extraction at $2/1000 pages ($1/1000 with Batch API).
  2000 pages/minute on a single node. Strong recommendation for
  production PDF pipelines that can tolerate a network hop.
- **`parser-llamaparse`** — LlamaIndex's cloud parser, LLM-powered.
  Strong on complex tables and mixed-layout documents.
- **`parser-unstructured-api`** — Hosted Unstructured.io. Same
  contract, no local compute.
- **`parser-reducto`** — Accuracy-focused commercial API, emerging
  player.
- **`parser-azure-di`** / **`parser-aws-textract`** / **`parser-google-docai`**
  — Cloud-native form/layout extraction for users already in those
  clouds.

**Note on Ollama.** Ollama is an LLM runtime, not a document parser. It
does not provide OCR or layout analysis as an API. Vision-language
models served *through* Ollama (llava, minicpm-v, qwen2-vl) can read
pages as images, but that is generic visual reasoning — not a parsing
pipeline with table structure preservation. For parsing, prefer a
real parser (local: LiteParse/Docling/Marker; online: Mistral OCR).
Ollama's job in this system is embeddings (`embedder-ollama`) and, at
the runtime layer, LLM inference.

### UI — file explorer in Work tab (ADR 0021)

Console route: `/docs`. Three-pane layout:

**Left — tree panel.**
- Collapsible folder tree, rendered from path strings.
- Icons encode `source`: human glyph for user docs, robot glyph for
  agent docs.
- Badge overlays: star for `knowledge: true`, dim/strike for
  `indexed: false`.
- Special nodes: `inbox/` pinned to top with unread count; `agents/<n>/`
  nodes show the agent avatar.
- Interactions: click to select, double-click to open in center pane,
  drag-and-drop to move, right-click for flag toggles (mark knowledge,
  toggle indexed, move, rename, delete).
- Top of tree: three filter toggles — `user`, `agent`, `knowledge`.
  Toggles control both tree visibility and search scope.

**Center — content pane.**
- Rendered preview per MIME:
  - markdown → rendered (same renderer as chat artifacts)
  - html/json/code → syntax-highlighted
  - pdf → pdf.js inline
  - images → inline
  - audio/video → native player
  - binary / unknown → metadata only with download button
- Toolbar: toggle knowledge flag, toggle indexed, edit (if
  markdown/text), move, rename, delete, download.

**Right — metadata pane.**
- `source`, `author`, `knowledge`, `indexed`, `writable_by_agents`
- `mime`, `size`, `created_at`, `updated_at`
- `chunk_count` (if indexed)
- **"Used by"** — last N agents that retrieved this doc via
  `DocsSearch`. This is the most valuable trust signal in the UI: users
  see which agents are actually reading their content.

**Top bar.**
- Unified search box — full-text + vector hits, merged and re-ranked,
  with snippet highlighting. Respects the active filter toggles.
- Upload button (routes to `inbox/` by default; user can pick folder).
- New-folder button.
- Filter toggles duplicated here for visibility when the tree is
  collapsed.

### API surface

Three consumers, one contract. The `DocsStore` + `DocsSearch` plugin
interface (internal) is rendered as HTTP endpoints by the daemon and as
CLI verbs by `zund`. Console talks HTTP; CLI talks HTTP; agents inside
containers talk HTTP (localhost on the per-agent socket per ADR 0008)
or use a Pi built-in tool that wraps HTTP.

**HTTP (daemon, under `/docs`):**

```
GET    /docs                          list (query: ?path=, ?source=, ?knowledge=,
                                              ?indexed=, ?tag.k=v, ?mime=, ?limit=, ?cursor=)
GET    /docs/tree                     folder tree (query: ?path=, ?depth=)
GET    /docs/:id                      metadata only
GET    /docs/:id/content              byte stream (Content-Type = doc.mime)
GET    /docs/:id/chunks               list chunks (debug, UI deep-link targets)
POST   /docs                          create — multipart: file + JSON metadata sidecar
                                      (path, tags, knowledge, indexed, writable_by_agents)
POST   /docs/ingest                   ingest from existing server-side path
                                      { path, parser?, chunker?, embedder? }
PATCH  /docs/:id                      partial update — flags, tags, custom, path
DELETE /docs/:id                      remove (blob gc'd if no refcount)
POST   /docs/:id/move                 { to: newPath } — metadata-only move
POST   /docs/search                   unified FTS + vector search with filters
                                      body: { q, filter: { source?, knowledge?, tags?,
                                              path_prefix?, page_range?, mime? },
                                              mode: "hybrid"|"fts"|"vector",
                                              limit, rerank? }
                                      → { hits: [{chunk, doc_meta, score, snippet}] }
GET    /docs/events  (SSE)            subscribe to doc-store events:
                                      doc.created, doc.updated, doc.deleted,
                                      doc.moved, inbox.arrived, ingest.progress
```

All endpoints respect the agent/user auth context from ADR 0020's auth
kind. `writable_by_agents: false` is enforced at `POST`/`PATCH`/`DELETE`
when the caller is an agent.

**CLI (`zund docs ...`):**

```
zund docs ls [path] [--source=user|agent] [--knowledge] [--tag k=v] [--tree]
zund docs tree [path] [--depth N]
zund docs get <path|id> [--content | --meta]             # stdout stream or JSON
zund docs put <file> [--to path] [--knowledge] [--tag k=v] [--no-index]
zund docs ingest <path> [--parser=liteparse|docling|...] [--chunker=...] [--force]
zund docs mv <src> <dst>
zund docs rm <path|id> [--recursive]
zund docs flag <path|id> [--knowledge=true|false] [--indexed=true|false]
                         [--writable-by-agents=true|false]
zund docs tag <path|id> k=v [k=v...]
zund docs untag <path|id> k [k...]
zund docs search <query> [--source=...] [--knowledge] [--tag k=v]
                         [--path <prefix>] [--mode=hybrid|fts|vector]
                         [--limit N] [--json]
zund docs info <path|id>                                 # full metadata dump
zund docs watch [path]                                   # tail SSE doc events
```

Everything else (`artifact/*` commands from ADR 0011) becomes a thin
deprecated alias: `zund artifact ls` → `zund docs ls --source=agent`,
prints a deprecation notice and forwards.

**Agent tool (Pi built-in, per ADR 0013):**

Agents running inside containers get built-in tools rather than raw
HTTP. These are the primary "write from agent" and "read from agent"
surfaces:

```
zund_docs_put(path, content, { mime?, tags?, knowledge? })
zund_docs_get(path_or_id) → { meta, content }
zund_docs_search(query, { filter?, mode?, limit? }) → hits[]
zund_docs_list(prefix, { source?, tags? }) → doc_meta[]
```

The agent's default write root is its `agents/<agent-name>/` prefix
(see reserved paths). Writing outside that prefix fails unless the
agent manifest declares explicit `output_paths:`. This is enforced at
the tool boundary inside Pi — agents cannot bypass via raw HTTP
because their socket auth is scoped to the agent identity.

**Contracts (package `@zund/core/contracts/docs.ts`):**

```ts
interface DocsStore {
  list(q: DocListQuery): Promise<DocMeta[]>;
  tree(path?: string, depth?: number): Promise<TreeNode>;
  get(idOrPath: string): Promise<DocMeta | null>;
  getContent(idOrPath: string): Promise<ReadableStream>;
  put(input: DocPutInput): Promise<DocMeta>;
  ingest(serverPath: string, opts?: IngestOpts): Promise<DocMeta>;
  patch(id: string, patch: DocPatch): Promise<DocMeta>;
  move(id: string, to: string): Promise<DocMeta>;
  remove(id: string): Promise<void>;
  chunks(id: string): Promise<Chunk[]>;
  subscribe(cb: (event: DocEvent) => void): Unsubscribe;
}

interface DocsSearch {
  query(req: SearchRequest): Promise<SearchHit[]>;
}
```

The plugin implementation (`docs-local`, `docs-s3`, etc.) satisfies
these. The daemon's HTTP and CLI layers are thin translators over the
registry-resolved plugin — they never import the implementation
directly (ADR 0020).

## Consequences

**Makes easier:**

- **Agent-to-agent data flow is zero-ceremony.** Researcher writes a
  file, blog-writer reads it via normal retrieval. No promote API, no
  handoff protocol.
- **Post-hoc promotion is a metadata flip, not a migration.** User
  reviews an agent draft, clicks "mark as knowledge," and it's done —
  same chunks, same embeddings, same URL.
- **Swapping parser / embedder / backend is a plugin install.** No
  changes to `docs` plugin, `agents`, or UI.
- **OSS ↔ Pro seam is clean.** Local filesystem + Ollama embeddings ships
  OSS; MinIO/S3 + managed vector DB + Mistral OCR is a Pro bundle.
  Contracts are shared.
- **UX matches user mental model.** "This is a file I want my agents to
  see" — no ontology lecture required.

**Makes harder:**

- **Model collapse risk if ignored.** Agents RAG-ing on their own past
  outputs can amplify hallucinations over time. Mitigated by retrieval
  policies that default to `source:user OR knowledge:true` for
  production flows, and let users explicitly opt into "all sources" for
  research modes.
- **Vector index cost grows faster** when artifacts are indexed by
  default. Mitigated by the `indexed` flag — agents can emit large
  transient outputs with `indexed: false` (e.g. raw logs) and let them
  expire via TTL.
- **Migration from ADR 0011.** Existing artifacts need a one-time
  migration: add `source: agent`, `knowledge: false`, `indexed: false`
  (don't force embed on migrate), slot paths under
  `agents/<name>/legacy/`. Addressed in implementation plan.
- **Permissions complexity.** `writable_by_agents: false` must be
  enforced at `DocsStore.put` for `source: user` docs. Simple rule,
  but another enforcement point.

## Implementation notes

- New package: `@zund/plugin-docs-local` (reference implementation).
  Owns its own `~/.zund/plugins/docs-local/` directory. Does not share
  SQLite with memory — independent plugin per ADR 0020.
- New contracts in `@zund/core/contracts/`: `docs.ts`, `parser.ts`,
  `chunker.ts`, `embedder.ts`, `vector-index.ts`.
- `kind: artifacts` in ADR 0020's service-tier list is superseded by
  `kind: docs`. ADR 0020 registry gets a minor update (not a
  supersession — the two-tier architecture stands).
- CLI surface: `zund docs ls | get | put | mv | rm | flag | search |
  ingest`. Existing `zund artifact` commands become thin wrappers that
  forward to `zund docs` with a provenance filter, then deprecate.
- Ingest watcher for `inbox/` is a v2 concern — v1 ships with explicit
  `zund docs ingest` and manual upload via console.
- Default parser policy: `parser-native` handles md/txt/html/json/csv/code
  out of the box. Other formats return `unsupported` at ingest time with
  a clear message pointing to contrib parser plugins. No silent
  best-effort text extraction.

## Open questions

- **Per-doc access control.** v1 assumes single-user or trusted-team.
  Multi-tenant ACLs (per-user read/write) are deferred to an access
  plugin kind — pairs with the auth pluggability in ADR 0020.
- **Full-text vs vector search blending.** Simple approach: run both,
  merge with a static weight. Better approach: rank via a reranker
  plugin. Start simple, add reranker plugin kind when needed.
- **Versioning.** Should docs have history (like a wiki)? Probably yes
  for user-authored docs, probably no for agent drafts (which churn
  heavily). Proposed: `versioned: bool` flag, off by default, writes are
  overwrites unless on.
- **Quota and TTL.** Retention per source, per path prefix? Inherit from
  fleet defaults? Aligns with ADR 0011's existing TTL sweeper — needs a
  design pass under the new model.
- **Embeddings-on-write vs background.** Ingest blocks on embed today;
  background queue is cleaner but adds a moving part. Decide based on
  UX of the upload flow.
