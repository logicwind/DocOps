package version

import (
	"strings"
	"testing"
)

func TestStringIncludesPrefix(t *testing.T) {
	got := String()
	if !strings.HasPrefix(got, "docops ") {
		t.Fatalf("String() = %q; want prefix %q", got, "docops ")
	}
}
