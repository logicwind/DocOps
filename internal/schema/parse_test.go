package schema

import (
	"errors"
	"strings"
	"testing"
)

func TestSplitFrontmatter_Basic(t *testing.T) {
	doc := []byte("---\ntitle: hi\n---\nbody text\n")
	fm, body, err := SplitFrontmatter(doc)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got, want := string(fm), "title: hi\n"; got != want {
		t.Fatalf("frontmatter = %q; want %q", got, want)
	}
	if got, want := string(body), "body text\n"; got != want {
		t.Fatalf("body = %q; want %q", got, want)
	}
}

func TestSplitFrontmatter_CRLF(t *testing.T) {
	doc := []byte("---\r\ntitle: hi\r\n---\r\nbody\r\n")
	fm, _, err := SplitFrontmatter(doc)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(string(fm), "title: hi") {
		t.Fatalf("frontmatter missing title: %q", fm)
	}
}

func TestSplitFrontmatter_Missing(t *testing.T) {
	_, _, err := SplitFrontmatter([]byte("no fence here\n"))
	if !errors.Is(err, ErrNoFrontmatter) {
		t.Fatalf("err = %v; want ErrNoFrontmatter", err)
	}
}

func TestSplitFrontmatter_Unterminated(t *testing.T) {
	_, _, err := SplitFrontmatter([]byte("---\ntitle: hi\nno closing fence\n"))
	if !errors.Is(err, ErrUnterminatedFrontmatter) {
		t.Fatalf("err = %v; want ErrUnterminatedFrontmatter", err)
	}
}

func TestParseRejectsUnknownField(t *testing.T) {
	// Context only has title/type/supersedes — `mystery` should fail strict decode.
	fm := []byte("title: x\ntype: memo\nmystery: 1\n")
	if _, err := ParseContext(fm); err == nil {
		t.Fatalf("expected error for unknown field")
	}
}

func TestParseTaskHappyPath(t *testing.T) {
	fm := []byte("title: x\nstatus: backlog\npriority: p1\nassignee: unassigned\nrequires: [ADR-0001]\ndepends_on: []\n")
	got, err := ParseTask(fm)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.Title != "x" || got.Status != "backlog" || len(got.Requires) != 1 {
		t.Fatalf("decoded wrong: %+v", got)
	}
}

func TestKindFromFilename(t *testing.T) {
	cases := []struct {
		name string
		want Kind
		ok   bool
	}{
		{"CTX-001-foo.md", KindContext, true},
		{"ADR-0012-foo.md", KindADR, true},
		{"TP-003-foo.md", KindTask, true},
		{"README.md", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, ok := KindFromFilename(tc.name)
		if got != tc.want || ok != tc.ok {
			t.Errorf("KindFromFilename(%q) = (%q,%v); want (%q,%v)", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}
