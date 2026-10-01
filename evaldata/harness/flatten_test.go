package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestMarkupLeak checks every trace that marks a sample as machine-written before a rater
// reads it, and that flat prose passes.
func TestMarkupLeak(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name string
		In   string
		Want string
	}{{ // Test 0: One flat line of prose carries nothing.
		Name: "flat", In: "A small tool that turns CSV into JSON, one row at a time.", Want: "",
	}, { // Test 1: A paragraph break is layout no human sample has.
		Name: "paragraph", In: "First paragraph.\n\nSecond paragraph.", Want: "a paragraph break",
	}, { // Test 2: A code span.
		Name: "code", In: "Run `csv2json` on the file.", Want: "a code span",
	}, { // Test 3: Bold.
		Name: "bold", In: "It is **fast** on big files.", Want: "emphasis markup",
	}, { // Test 4: Link syntax, including the half a wrapped link leaves.
		Name: "link", In: "See [the docs]( for more.", Want: "link syntax",
	}, { // Test 5: Table and HTML characters.
		Name: "table", In: "a | b | c", Want: "table or HTML markup",
	}, { // Test 6: The doubled space a stripped span leaves behind.
		Name: "residue", In: "It uses  throughout the code.", Want: "a doubled space",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(test.Want, markupLeak(test.In)); diff != "" {
				t.Errorf("markupLeak mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestNextID checks that a dropped sample never hands its number to a new one.
func TestNextID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name    string
		Samples []Sample
		Prefix  string
		WantID  int
	}{{ // Test 0: An empty corpus starts at one.
		Name: "empty", Prefix: "a", WantID: 1,
	}, { // Test 1: A gap in the middle does not lower the next number.
		Name: "gap", Prefix: "a", WantID: 6,
		Samples: []Sample{{ID: "a001"}, {ID: "a005"}, {ID: "a003"}},
	}, { // Test 2: The other half's ids do not count.
		Name: "other prefix", Prefix: "h", WantID: 3,
		Samples: []Sample{{ID: "a009"}, {ID: "h002"}},
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(test.WantID, nextID(test.Samples, test.Prefix)); diff != "" {
				t.Errorf("nextID mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// writeCorpus writes samples to a fresh corpus file and returns its path.
func writeCorpus(t *testing.T, samples []Sample) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "samples.jsonl")
	if err := writeSamples(path, samples); err != nil {
		t.Fatalf("writeSamples: %v", err)
	}
	return path
}

// TestFlattenCorpus checks that a marked sample is flattened to the human half's form, a
// sample that is mostly structure is dropped, a clean one is left alone, and the result
// passes the corpus check that failed before.
func TestFlattenCorpus(t *testing.T) {
	t.Parallel()
	prose := words(80)
	clean := Sample{ID: "h001", Source: "human", Rules: "v1", Text: prose}
	marked := Sample{ID: "a001", Source: "ai", Rules: "v1",
		Text: "# csv2json\n\n" + prose + "\n\n" + "It is **fast** and uses `io.Reader` throughout the code."}
	listy := Sample{ID: "a002", Source: "ai", Rules: "v1",
		Text: "# Tool\n\n" + strings.Repeat("- a bullet point item here\n", 30)}
	path := writeCorpus(t, []Sample{clean, marked, listy})
	if got := checkCorpus([]Sample{clean, marked, listy}, nil); len(got) != 2 {
		t.Fatalf("before flattening: problems = %d, want 2: %v", len(got), got)
	}

	var out strings.Builder
	if err := flattenCorpus(path, filepath.Join(t.TempDir(), "ratings.jsonl"), &out); err != nil {
		t.Fatalf("flattenCorpus: %v", err)
	}
	got, err := readLines[Sample](path)
	if err != nil {
		t.Fatalf("readLines: %v", err)
	}
	want := []Sample{clean, {ID: "a001", Source: "ai", Rules: "v1",
		Text: prose + " It is fast and uses throughout the code."}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("corpus mismatch (-want +got):\n%s", diff)
	}
	if problems := checkCorpus(got, nil); len(problems) != 0 {
		t.Errorf("after flattening: problems = %v, want none", problems)
	}
	if !strings.Contains(out.String(), "a002 dropped") {
		t.Errorf("the run did not report the dropped sample:\n%s", out.String())
	}
}

// TestFlattenCorpusRefusesRated checks that a rated corpus is never rewritten, since a
// rating belongs to the exact text the rater read.
func TestFlattenCorpusRefusesRated(t *testing.T) {
	t.Parallel()
	marked := Sample{ID: "a001", Source: "ai", Rules: "v1", Text: words(80) + "\n\n" + words(20)}
	path := writeCorpus(t, []Sample{marked})
	ratings := filepath.Join(t.TempDir(), "ratings.jsonl")
	if err := os.WriteFile(ratings, []byte(`{"sample":"a001","rater":"r01","machine":5}`+"\n"), 0o600); err != nil {
		t.Fatalf("write ratings: %v", err)
	}
	if err := flattenCorpus(path, ratings, io.Discard); err == nil {
		t.Fatal("flattenCorpus rewrote a rated corpus, want an error")
	}
	got, err := readLines[Sample](path)
	if err != nil {
		t.Fatalf("readLines: %v", err)
	}
	if diff := cmp.Diff([]Sample{marked}, got); diff != "" {
		t.Errorf("corpus changed (-want +got):\n%s", diff)
	}
}

// TestGenerateAIFromFlattens checks that a generated reply is stored in the human half's
// form, with no heading, list, or paragraph break for a rater to read the label from.
func TestGenerateAIFromFlattens(t *testing.T) {
	t.Parallel()
	reply := "# csv2json\n\n" + words(70) + "\n\n## Install\n\n- go install it\n\n" + words(20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, fmt.Sprintf(`{"response":%q}`, reply))
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "samples.jsonl")
	if err := generateAIFrom(srv.URL, []string{"m"}, []string{"readme"}, 1, path, "v1", io.Discard); err != nil {
		t.Fatalf("generateAIFrom: %v", err)
	}
	got, err := readLines[Sample](path)
	if err != nil {
		t.Fatalf("readLines: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("no samples written")
	}
	for _, s := range got {
		if leak := markupLeak(s.Text); leak != "" {
			t.Errorf("sample %s carries %s: %q", s.ID, leak, s.Text)
		}
	}
}
