package schema

import (
	"errors"
	"strings"
	"testing"
)

var cfgWithTypes = Config{ContextTypes: []string{"prd", "memo", "research", "notes", "brief", "spec", "design"}}

func TestValidateContext_Happy(t *testing.T) {
	c := Context{Title: "hi", Type: "memo"}
	if err := ValidateContext(c, cfgWithTypes); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateContext_UnknownType(t *testing.T) {
	c := Context{Title: "hi", Type: "novel"}
	err := ValidateContext(c, cfgWithTypes)
	if err == nil || !strings.Contains(err.Error(), "type:") {
		t.Fatalf("expected type error, got %v", err)
	}
}

func TestValidateContext_SkipEnumWhenNoConfig(t *testing.T) {
	// Allows tools to validate CTX files without a loaded config.
	c := Context{Title: "hi", Type: "anything"}
	if err := ValidateContext(c, Config{}); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateADR_Happy(t *testing.T) {
	a := ADR{Title: "x", Status: "accepted", Coverage: "required", Date: "2026-04-22"}
	if err := ValidateADR(a); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateADR_BadEnum(t *testing.T) {
	a := ADR{Title: "x", Status: "in-review", Coverage: "required", Date: "2026-04-22"}
	err := ValidateADR(a)
	if err == nil || !strings.Contains(err.Error(), "status:") {
		t.Fatalf("expected status enum error, got %v", err)
	}
}

func TestValidateADR_BadDate(t *testing.T) {
	a := ADR{Title: "x", Status: "accepted", Coverage: "required", Date: "yesterday"}
	err := ValidateADR(a)
	if err == nil || !strings.Contains(err.Error(), "date:") {
		t.Fatalf("expected date error, got %v", err)
	}
}

func TestValidateADR_BadRef(t *testing.T) {
	a := ADR{Title: "x", Status: "accepted", Coverage: "required", Date: "2026-04-22", Related: []string{"adr-0002"}}
	err := ValidateADR(a)
	if err == nil || !strings.Contains(err.Error(), "related[0]") {
		t.Fatalf("expected ref-pattern error, got %v", err)
	}
}

func TestValidateTask_Happy(t *testing.T) {
	task := Task{Title: "x", Status: "backlog", Priority: "p1", Requires: []string{"ADR-0001"}}
	if err := ValidateTask(task); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateTask_EmptyRequiresFails(t *testing.T) {
	task := Task{Title: "x", Status: "backlog", Requires: []string{}}
	err := ValidateTask(task)
	if err == nil || !strings.Contains(err.Error(), "requires:") {
		t.Fatalf("expected requires error, got %v", err)
	}
}

func TestValidateTask_OnlyTaskRefsFailsADR0004(t *testing.T) {
	task := Task{Title: "x", Status: "backlog", Requires: []string{"TP-002"}}
	err := ValidateTask(task)
	if err == nil || !strings.Contains(err.Error(), "ADR-0004") {
		t.Fatalf("expected ADR-0004 alignment error, got %v", err)
	}
}

func TestValidateTask_BadStatus(t *testing.T) {
	task := Task{Title: "x", Status: "in-progress", Requires: []string{"ADR-0001"}}
	err := ValidateTask(task)
	if err == nil || !strings.Contains(err.Error(), "status:") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestValidationErrors_ReportsAll(t *testing.T) {
	task := Task{} // empty: title, status, requires all fail.
	err := ValidateTask(task)
	if err == nil {
		t.Fatalf("expected multiple errors")
	}
	var ve ValidationErrors
	if !errors.As(err, &ve) {
		t.Fatalf("err is not ValidationErrors: %T", err)
	}
	if len(ve) < 3 {
		t.Fatalf("expected ≥3 field errors, got %d: %v", len(ve), ve)
	}
}
