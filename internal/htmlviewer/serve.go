package htmlviewer

import (
	"encoding/json"
	"fmt"
	"net/http"
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
//	GET /             → the embedded SPA
//	GET /index.html   → same
//	GET /index.json   → fresh viewer bundle (index graph + doc bodies + STATE.md)
//	GET /health       → {"status":"ok"}
//
// Root is the project root (directory containing docops.yaml). Cfg is the
// loaded project configuration.
func Handler(root string, cfg config.Config) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only serve the SPA for the root path; 404 for anything else not
		// claimed by a more specific route.
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(SPA)
	})

	mux.HandleFunc("/index.json", func(w http.ResponseWriter, _ *http.Request) {
		idx, err := buildIndex(root, cfg)
		if err != nil {
			http.Error(w, fmt.Sprintf("build index: %v", err), http.StatusInternalServerError)
			return
		}
		bundle, err := BuildBundle(idx, cfg, root)
		if err != nil {
			http.Error(w, fmt.Sprintf("build bundle: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(bundle)
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
