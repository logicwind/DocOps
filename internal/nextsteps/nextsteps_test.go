package nextsteps

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRender_Empty(t *testing.T) {
	var b bytes.Buffer
	Render(&b, nil)
	if b.Len() != 0 {
		t.Fatalf("Render(nil) wrote %q, want empty", b.String())
	}
}

func TestRender_Format(t *testing.T) {
	var b bytes.Buffer
	Render(&b, []Step{
		{Label: "validate + index + state", Command: "docops refresh"},
	})
	out := b.String()
	if !strings.Contains(out, "→ Next:") {
		t.Errorf("missing header in %q", out)
	}
	if !strings.Contains(out, "• docops refresh") {
		t.Errorf("missing command bullet in %q", out)
	}
	if !strings.Contains(out, "validate + index + state") {
		t.Errorf("missing label in %q", out)
	}
}

func TestEncodeJSON_RoundTrip(t *testing.T) {
	steps := []Step{
		{Label: "x", Command: "docops y"},
		{Label: "", Command: "docops z"},
	}
	raw := EncodeJSON(steps)

	var got []Step
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 2 || got[0].Command != "docops y" || got[1].Command != "docops z" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestSuggest_VerbsCoverage(t *testing.T) {
	tests := []struct {
		name string
		got  []Step
	}{
		{"init-greenfield", ForInit(Outcome{Brownfield: false})},
		{"init-brownfield", ForInit(Outcome{Brownfield: true})},
		{"new-ctx", ForNewCTX(Outcome{ID: "CTX-001"})},
		{"new-adr", ForNewADR(Outcome{ID: "ADR-0042"})},
		{"new-task", ForNewTask(Outcome{ID: "TP-099"})},
		{"refresh", ForRefresh(Outcome{})},
		{"amend", ForAmend(Outcome{ID: "ADR-0042"})},
		{"audit-gaps", ForAudit(Outcome{HasGaps: true})},
		{"audit-clean", ForAudit(Outcome{HasGaps: false})},
		{"state", ForState(Outcome{})},
		{"upgrade", ForUpgrade(Outcome{})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.got) == 0 {
				t.Fatalf("verb %s returned no steps", tt.name)
			}
			if len(tt.got) > 3 {
				t.Fatalf("verb %s returned %d steps, max 3", tt.name, len(tt.got))
			}
			for i, s := range tt.got {
				if s.Command == "" {
					t.Errorf("step %d has empty Command", i)
				}
			}
		})
	}
}

func TestSuggest_NewCTXMentionsID(t *testing.T) {
	steps := ForNewCTX(Outcome{ID: "CTX-007"})
	var any bool
	for _, s := range steps {
		if strings.Contains(s.Command, "CTX-007") {
			any = true
		}
	}
	if !any {
		t.Fatalf("ForNewCTX did not mention the new ID in any command: %+v", steps)
	}
}
