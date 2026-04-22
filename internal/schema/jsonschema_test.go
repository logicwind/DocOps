package schema

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONSchemas_AllThreePresent(t *testing.T) {
	schemas, err := JSONSchemas(Config{ContextTypes: []string{"memo"}})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	for _, name := range []string{"context.schema.json", "decision.schema.json", "task.schema.json"} {
		raw, ok := schemas[name]
		if !ok {
			t.Fatalf("missing %s", name)
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			t.Fatalf("%s is not valid JSON: %v", name, err)
		}
		if obj["$schema"] != JSONSchemaVersion {
			t.Errorf("%s $schema = %v; want %s", name, obj["$schema"], JSONSchemaVersion)
		}
	}
}

func TestContextSchema_EnumPassthrough(t *testing.T) {
	schemas, err := JSONSchemas(Config{ContextTypes: []string{"prd", "memo"}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(schemas["context.schema.json"])
	if !strings.Contains(s, `"prd"`) || !strings.Contains(s, `"memo"`) {
		t.Fatalf("context schema missing enum values:\n%s", s)
	}
}

func TestTaskSchema_RequiresHasContainsClause(t *testing.T) {
	// ADR-0004: `requires` must include at least one CTX or ADR reference.
	// JSON Schema's `contains` keyword encodes that without custom code.
	schemas, err := JSONSchemas(Config{})
	if err != nil {
		t.Fatal(err)
	}
	s := string(schemas["task.schema.json"])
	if !strings.Contains(s, `"contains"`) {
		t.Fatalf("task schema missing `contains` clause for ADR-0004:\n%s", s)
	}
	if !strings.Contains(s, `^(CTX|ADR)-`) {
		t.Fatalf("task schema contains pattern missing CTX|ADR restriction:\n%s", s)
	}
}

func TestJSONSchemas_IDMatchesFilename(t *testing.T) {
	schemas, err := JSONSchemas(Config{})
	if err != nil {
		t.Fatal(err)
	}
	for name, raw := range schemas {
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		id, _ := obj["$id"].(string)
		if !strings.HasSuffix(id, "/"+name) {
			t.Errorf("%s: $id %q does not end with /%s", name, id, name)
		}
	}
}

func TestADRSchema_EnumsMatchGoConstants(t *testing.T) {
	schemas, err := JSONSchemas(Config{})
	if err != nil {
		t.Fatal(err)
	}
	s := string(schemas["decision.schema.json"])
	for _, v := range ADRStatuses {
		if !strings.Contains(s, `"`+v+`"`) {
			t.Errorf("adr schema missing status %q", v)
		}
	}
	for _, v := range ADRCoverages {
		if !strings.Contains(s, `"`+v+`"`) {
			t.Errorf("adr schema missing coverage %q", v)
		}
	}
}
