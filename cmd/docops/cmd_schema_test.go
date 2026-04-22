package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withDocopsYAML writes a minimal docops.yaml into root and returns root.
func withDocopsYAML(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "docops.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write docops.yaml: %v", err)
	}
}

func TestCmdSchema_NoConfig_ExitsTwo(t *testing.T) {
	root := t.TempDir()
	// Change into an empty tempdir so FindAndLoad finds nothing.
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	code := cmdSchema(nil)
	if code != 2 {
		t.Errorf("expected exit 2 without docops.yaml, got %d", code)
	}
}

func TestCmdSchema_WritesThreeFiles(t *testing.T) {
	root := t.TempDir()
	withDocopsYAML(t, root, "version: 1\ncontext_types: [prd, memo]\n")

	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	code := cmdSchema(nil)
	if code != 0 {
		t.Fatalf("cmdSchema returned %d", code)
	}

	for _, name := range []string{"context.schema.json", "decision.schema.json", "task.schema.json"} {
		path := filepath.Join(root, "docs/.docops/schema", name)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("missing %s: %v", name, err)
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(body, &obj); err != nil {
			t.Errorf("%s is not valid JSON: %v", name, err)
		}
	}
}

func TestCmdSchema_Stdout_ParseableJSON(t *testing.T) {
	root := t.TempDir()
	withDocopsYAML(t, root, "version: 1\ncontext_types: [alpha, beta]\n")

	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Redirect stdout to capture output.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStdout := os.Stdout
	os.Stdout = w

	code := cmdSchema([]string{"--stdout"})

	_ = w.Close()
	os.Stdout = origStdout

	if code != 0 {
		t.Fatalf("cmdSchema --stdout returned %d", code)
	}

	buf := make([]byte, 1<<16)
	n, _ := r.Read(buf)
	out := string(buf[:n])

	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("--stdout output is not valid JSON: %v\n%s", err, out)
	}
	for _, name := range []string{"context.schema.json", "decision.schema.json", "task.schema.json"} {
		if _, ok := obj[name]; !ok {
			t.Errorf("--stdout output missing key %q", name)
		}
	}
	// context_types must be reflected in context schema.
	ctxRaw := string(obj["context.schema.json"])
	if !strings.Contains(ctxRaw, `"alpha"`) || !strings.Contains(ctxRaw, `"beta"`) {
		t.Errorf("context schema missing custom enum values: %s", ctxRaw)
	}
}

func TestCmdSchema_Idempotent(t *testing.T) {
	root := t.TempDir()
	withDocopsYAML(t, root, "version: 1\n")

	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if code := cmdSchema(nil); code != 0 {
		t.Fatalf("first run: %d", code)
	}
	if code := cmdSchema(nil); code != 0 {
		t.Fatalf("second run: %d", code)
	}

	// Files must exist and be valid JSON after both runs.
	for _, name := range []string{"context.schema.json", "decision.schema.json", "task.schema.json"} {
		body, err := os.ReadFile(filepath.Join(root, "docs/.docops/schema", name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(body, &obj); err != nil {
			t.Errorf("%s invalid JSON: %v", name, err)
		}
	}
}
