package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/logicwind/docops/internal/config"
	"github.com/logicwind/docops/internal/index"
	"github.com/logicwind/docops/internal/loader"
	"github.com/logicwind/docops/internal/validator"
)

// bootstrapIndex is the shared bootstrap sequence for read-only commands:
// find config, load docs, validate, build in-memory index.
// On any failure it prints a prefixed error to stderr and returns code 2.
func bootstrapIndex(cmd string) (*index.Index, int) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops %s: %v\n", cmd, err)
		return nil, 2
	}
	cfg, root, err := config.FindAndLoad(cwd)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "docops %s: no docops.yaml found — run `docops init` first\n", cmd)
			return nil, 2
		}
		fmt.Fprintf(os.Stderr, "docops %s: %v\n", cmd, err)
		return nil, 2
	}
	set, err := loader.Load(root, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops %s: %v\n", cmd, err)
		return nil, 2
	}
	report := validator.Validate(set, cfg)
	if !report.OK() {
		fmt.Fprintf(os.Stderr, "docops %s: refusing: %d validation error(s); run 'docops validate'\n", cmd, len(report.Errors))
		return nil, 2
	}
	idx, err := index.Build(set, cfg, root, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "docops %s: build index: %v\n", cmd, err)
		return nil, 2
	}
	return idx, 0
}

// indexLookup finds a doc by ID.
func indexLookup(idx *index.Index, id string) (index.IndexedDoc, bool) {
	for _, doc := range idx.Docs {
		if doc.ID == id {
			return doc, true
		}
	}
	return index.IndexedDoc{}, false
}

// indexByID builds a map from ID → IndexedDoc for O(1) lookup.
func indexByID(idx *index.Index) map[string]index.IndexedDoc {
	m := make(map[string]index.IndexedDoc, len(idx.Docs))
	for _, doc := range idx.Docs {
		m[doc.ID] = doc
	}
	return m
}
