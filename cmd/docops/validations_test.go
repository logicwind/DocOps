package main

import "testing"

func TestValidKind(t *testing.T) {
	for _, k := range []string{"CTX", "ADR", "TP"} {
		if !validKind(k) {
			t.Errorf("validKind(%q) = false, want true", k)
		}
	}
	for _, k := range []string{"task", "decision", "context", "CTX ", "", "adr", "ADRS", "TP-001"} {
		if validKind(k) {
			t.Errorf("validKind(%q) = true, want false", k)
		}
	}
}