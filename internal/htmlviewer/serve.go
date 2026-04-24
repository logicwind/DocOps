package htmlviewer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/index"
	"github.com/logicwind/docops/internal/loader"
	"github.com/logicwind/docops/internal/validator"
)

// Handler returns the HTTP handler used by `docops serve`.
//
// Routes:
//
//	GET /               → the embedded SPA
//	GET /index.html     → same
//	GET /index.json     → fresh index built via internal/index.Build
//	GET /state.md       → contents of cfg.Paths.State, or "" if absent
//	GET /raw/<path>     → raw markdown file under docs/ (traversal-safe)
//	GET /health         → {"status":"ok"}
//
// Root is the project root (directory containing docops.yaml). Cfg is the
// loaded project configuration.
func Handler(root string, cfg config.Config) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Catch-all on "/" — only serve the SPA for the root path,
		// 404 for anything else not claimed by a more specific route.
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(SPA)
	})

	mux.HandleFunc("/index.json", func(w http.ResponseWriter, r *http.Request) {
		idx, err := buildIndex(root, cfg)
		if err != nil {
			http.Error(w, fmt.Sprintf("build index: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(idx)
	})

	mux.HandleFunc("/state.md", func(w http.ResponseWriter, r *http.Request) {
		stateSrc := cfg.Paths.State
		if stateSrc == "" {
			stateSrc = "docs/STATE.md"
		}
		if !filepath.IsAbs(stateSrc) {
			stateSrc = filepath.Join(root, stateSrc)
		}
		b, err := os.ReadFile(stateSrc)
		if err != nil {
			if os.IsNotExist(err) {
				w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
				_, _ = w.Write(nil)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(b)
	})

	mux.HandleFunc("/raw/", func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/raw/")
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" || strings.Contains(rel, "..") {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		clean := filepath.Clean(rel)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		// Only serve files under the configured docs/* folders.
		allowed := []string{cfg.Paths.Context, cfg.Paths.Decisions, cfg.Paths.Tasks}
		prefixOK := false
		for _, a := range allowed {
			if a != "" && (clean == a || strings.HasPrefix(clean, a+string(filepath.Separator))) {
				prefixOK = true
				break
			}
		}
		if !prefixOK {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		abs := filepath.Join(root, clean)
		b, err := os.ReadFile(abs)
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(b)
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	return mux
}

// buildIndex is the serve equivalent of cmd/docops/bootstrap.bootstrapIndex —
// pulled into this package so cmd_serve.go stays thin.
func buildIndex(root string, cfg config.Config) (*index.Index, error) {
	set, err := loader.Load(root, cfg)
	if err != nil {
		return nil, err
	}
	report := validator.Validate(set, cfg)
	if !report.OK() {
		return nil, fmt.Errorf("validation failed: %d error(s); run 'docops validate'", len(report.Errors))
	}
	return index.Build(set, cfg, root, time.Now())
}
