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
	// Allowed values must appear in the message.
	if !strings.Contains(err.Error(), "memo") {
		t.Errorf("expected allowed values in message, got %v", err)
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
	// Allowed values must appear in the message.
	if !strings.Contains(err.Error(), "draft") || !strings.Contains(err.Error(), "accepted") {
		t.Errorf("expected allowed values in enum error message, got %v", err)
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

// TestEnumError_IncludesAllowedValues asserts that every invalid-enum finding
// includes the full allowed-values list in its message — so agents can fix the
// value on first read without a follow-up round-trip (ADR-0020).
func TestEnumError_IncludesAllowedValues(t *testing.T) {
	t.Run("ADR status", func(t *testing.T) {
		a := ADR{Title: "x", Status: "proposed", Coverage: "required", Date: "2026-04-22"}
		err := ValidateADR(a)
		if err == nil {
			t.Fatal("expected error")
		}
		for _, v := range ADRStatuses {
			if !strings.Contains(err.Error(), v) {
				t.Errorf("allowed value %q missing from message: %v", v, err)
			}
		}
	})

	t.Run("ADR coverage", func(t *testing.T) {
		a := ADR{Title: "x", Status: "draft", Coverage: "optional", Date: "2026-04-22"}
		err := ValidateADR(a)
		if err == nil {
			t.Fatal("expected error")
		}
		for _, v := range ADRCoverages {
			if !strings.Contains(err.Error(), v) {
				t.Errorf("allowed value %q missing from message: %v", v, err)
			}
		}
	})

	t.Run("Task status", func(t *testing.T) {
		task := Task{Title: "x", Status: "in-progress", Priority: "p1", Requires: []string{"ADR-0001"}}
		err := ValidateTask(task)
		if err == nil {
			t.Fatal("expected error")
		}
		for _, v := range TaskStatuses {
			if !strings.Contains(err.Error(), v) {
				t.Errorf("allowed value %q missing from message: %v", v, err)
			}
		}
	})

	t.Run("Task priority", func(t *testing.T) {
		task := Task{Title: "x", Status: "backlog", Priority: "high", Requires: []string{"ADR-0001"}}
		err := ValidateTask(task)
		if err == nil {
			t.Fatal("expected error")
		}
		for _, v := range TaskPriorities {
			if !strings.Contains(err.Error(), v) {
				t.Errorf("allowed value %q missing from message: %v", v, err)
			}
		}
	})

	t.Run("CTX type with config", func(t *testing.T) {
		cfg := Config{ContextTypes: []string{"prd", "memo", "brief"}}
		c := Context{Title: "x", Type: "unknown"}
		err := ValidateContext(c, cfg)
		if err == nil {
			t.Fatal("expected error")
		}
		for _, v := range cfg.ContextTypes {
			if !strings.Contains(err.Error(), v) {
				t.Errorf("allowed value %q missing from message: %v", v, err)
			}
		}
	})
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

// --- ADR-0025 amendments ---------------------------------------------------

func validADR() ADR {
	return ADR{Title: "x", Status: "accepted", Coverage: "required", Date: "2026-04-22"}
}

func TestValidateADR_AmendmentHappy(t *testing.T) {
	a := validADR()
	a.Amendments = []Amendment{{Date: "2026-04-23", Kind: "editorial", By: "nix", Summary: "fix typo"}}
	if err := ValidateADR(a); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateADR_AmendmentBadKind(t *testing.T) {
	a := validADR()
	a.Amendments = []Amendment{{Date: "2026-04-23", Kind: "rewrite", By: "nix", Summary: "x"}}
	err := ValidateADR(a)
	if err == nil || !strings.Contains(err.Error(), "amendments[0].kind") {
		t.Fatalf("expected amendments[0].kind error, got %v", err)
	}
	for _, want := range AmendmentKinds {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q in message: %v", want, err)
		}
	}
}

func TestValidateADR_AmendmentMissingFields(t *testing.T) {
	a := validADR()
	a.Amendments = []Amendment{{}} // everything empty
	err := ValidateADR(a)
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, want := range []string{"amendments[0].date", "amendments[0].kind", "amendments[0].by", "amendments[0].summary"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q in message: %v", want, err)
		}
	}
}

func TestValidateAmendmentMarkers_HappyMatch(t *testing.T) {
	body := []byte("the foo [AMENDED 2026-04-23 editorial] is now bar.\n")
	amends := []Amendment{{Date: "2026-04-23", Kind: "editorial", By: "nix", Summary: "x"}}
	if errs := ValidateAmendmentMarkers(amends, body); len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateAmendmentMarkers_StrayMarker(t *testing.T) {
	body := []byte("the foo [AMENDED 2026-04-23 editorial] is now bar.\n")
	errs := ValidateAmendmentMarkers(nil, body)
	if len(errs) == 0 || !strings.Contains(errs.Error(), "no matching frontmatter entry") {
		t.Fatalf("expected stray-marker error, got %v", errs)
	}
}

func TestValidateAmendmentMarkers_FrontmatterWithoutMarkerOrSections(t *testing.T) {
	body := []byte("nothing relevant here.\n")
	amends := []Amendment{{Date: "2026-04-23", Kind: "editorial", By: "nix", Summary: "x"}}
	errs := ValidateAmendmentMarkers(amends, body)
	if len(errs) == 0 || !strings.Contains(errs.Error(), "neither an inline [AMENDED] marker nor affects_sections") {
		t.Fatalf("expected unanchored-entry error, got %v", errs)
	}
}

func TestValidateAmendmentMarkers_AffectsSectionsAnchorOK(t *testing.T) {
	body := []byte("nothing inline here.\n")
	amends := []Amendment{{Date: "2026-04-23", Kind: "editorial", By: "nix", Summary: "x", AffectsSections: []string{"Decision"}}}
	if errs := ValidateAmendmentMarkers(amends, body); len(errs) != 0 {
		t.Fatalf("expected no errors with affects_sections, got %v", errs)
	}
}

func TestValidateAmendmentMarkers_SkipsFencedCodeBlocks(t *testing.T) {
	body := []byte("```\nexample: foo [AMENDED 2026-04-23 editorial] bar\n```\n\nactual prose.\n")
	if errs := ValidateAmendmentMarkers(nil, body); len(errs) != 0 {
		t.Fatalf("expected no errors (marker is in fenced code), got %v", errs)
	}
}

func TestValidateAmendmentMarkers_KindMismatch(t *testing.T) {
	body := []byte("foo [AMENDED 2026-04-23 errata] bar.\n")
	amends := []Amendment{{Date: "2026-04-23", Kind: "editorial", By: "nix", Summary: "x"}}
	errs := ValidateAmendmentMarkers(amends, body)
	if len(errs) < 2 {
		t.Fatalf("expected stray-marker + unanchored-entry errors, got %v", errs)
	}
}
