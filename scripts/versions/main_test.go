package main

import (
	"strings"
	"testing"
)

// TestCollect runs against the real repository files, so a change in a
// workflow that breaks a pattern fails here.
func TestCollect(t *testing.T) {
	t.Chdir("../..")

	rows, err := collect()
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	for _, r := range rows {
		if r.version == "" {
			t.Errorf("%s: empty version", r.component)
		}
	}
}

func TestSecurityFileIsUpToDate(t *testing.T) {
	t.Chdir("../..")
	if err := run(true); err != nil {
		t.Fatal(err)
	}
}

func TestDirectDeps(t *testing.T) {
	goMod := []byte(`module example.com/m

go 1.26.0

require example.com/single v1.0.0

require (
	example.com/direct v1.2.3
	example.com/indirect v0.1.0 // indirect
)
`)
	got := directDeps(goMod)
	want := []row{{"example.com/single", "v1.0.0"}, {"example.com/direct", "v1.2.3"}}
	if len(got) != len(want) {
		t.Fatalf("directDeps() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("dep #%d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestReplaceTable(t *testing.T) {
	doc := []byte("before\n" + startMarker + "\nold\n" + endMarker + "\nafter\n")
	got, err := replaceTable(doc, "new\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "before\n" + startMarker + "\nnew\n" + endMarker + "\nafter\n"
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	if _, err := replaceTable([]byte("no markers"), "x"); err == nil {
		t.Fatal("expected an error without markers")
	}
}

func TestExtractorReportsMissingVersion(t *testing.T) {
	var e extractor
	e.find([]byte("nothing here"), "tool", `tool@(v\S+)`)
	if e.err == nil || !strings.Contains(e.err.Error(), "tool") {
		t.Fatalf("err = %v, want an error naming the tool", e.err)
	}
}
