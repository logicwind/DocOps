// Package config loads the project-level `docops.yaml` file. The full
// schema is generous — phase 1 only reads the fields the current commands
// need. Unknown fields are ignored so older binaries coexist with newer
// config without breaking.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultFilename is the canonical name for the project config.
const DefaultFilename = "docops.yaml"

// Config is the in-memory representation of docops.yaml. Fields are added
// as commands need them; keep the struct minimal.
type Config struct {
	Version      int      `yaml:"version"`
	Paths        Paths    `yaml:"paths"`
	ContextTypes []string `yaml:"context_types"`
}

// Paths mirrors the `paths:` section of docops.yaml. Zero values get
// sensible defaults via ApplyDefaults so partial configs work.
type Paths struct {
	Context   string `yaml:"context"`
	Decisions string `yaml:"decisions"`
	Tasks     string `yaml:"tasks"`
	Index     string `yaml:"index"`
	State     string `yaml:"state"`
	Counters  string `yaml:"counters"`
	Schema    string `yaml:"schema"`
	Reviews   string `yaml:"reviews"`
}

// Default returns a Config with the hard-coded defaults used when no
// docops.yaml is present. These match templates/docops.yaml.tmpl.
func Default() Config {
	return Config{
		Version: 1,
		Paths: Paths{
			Context:   "docs/context",
			Decisions: "docs/decisions",
			Tasks:     "docs/tasks",
			Index:     "docs/.index.json",
			State:     "docs/STATE.md",
			Counters:  "docs/.docops/counters.json",
			Schema:    "docs/.docops/schema",
			Reviews:   "docs/decisions/.reviews",
		},
		ContextTypes: []string{"prd", "design", "research", "notes", "memo", "spec", "brief"},
	}
}

// ApplyDefaults fills in any zero-valued fields from Default().
func (c *Config) ApplyDefaults() {
	def := Default()
	if c.Version == 0 {
		c.Version = def.Version
	}
	if c.Paths.Context == "" {
		c.Paths.Context = def.Paths.Context
	}
	if c.Paths.Decisions == "" {
		c.Paths.Decisions = def.Paths.Decisions
	}
	if c.Paths.Tasks == "" {
		c.Paths.Tasks = def.Paths.Tasks
	}
	if c.Paths.Index == "" {
		c.Paths.Index = def.Paths.Index
	}
	if c.Paths.State == "" {
		c.Paths.State = def.Paths.State
	}
	if c.Paths.Counters == "" {
		c.Paths.Counters = def.Paths.Counters
	}
	if c.Paths.Schema == "" {
		c.Paths.Schema = def.Paths.Schema
	}
	if c.Paths.Reviews == "" {
		c.Paths.Reviews = def.Paths.Reviews
	}
	if len(c.ContextTypes) == 0 {
		c.ContextTypes = def.ContextTypes
	}
}

// Load reads and parses docops.yaml from the given path. Missing files
// produce os.ErrNotExist — callers that want the "use defaults" behaviour
// should catch that and call Default() themselves.
func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.ApplyDefaults()
	return cfg, nil
}

// FindAndLoad walks up from startDir looking for docops.yaml and returns
// the loaded config plus the directory it was found in (the project root).
// Returns os.ErrNotExist if no config is found above the filesystem root.
func FindAndLoad(startDir string) (Config, string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return Config{}, "", err
	}
	for {
		candidate := filepath.Join(dir, DefaultFilename)
		if _, err := os.Stat(candidate); err == nil {
			cfg, loadErr := Load(candidate)
			return cfg, dir, loadErr
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Config{}, "", os.ErrNotExist
		}
		dir = parent
	}
}
