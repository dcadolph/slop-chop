package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/dcadolph/slop-chop/sanitize"
)

// longProse returns a passage of n sentences with varied length, which is what the corpus
// filters are written to accept. Building it rather than pasting a literal keeps the
// eligibility boundary readable in the tests that sit on it.
func longProse(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("The service reads the queue and writes a record for every job it finishes")
		if i%3 == 0 {
			b.WriteString(", though the writer batches them when the queue runs long")
		}
		b.WriteString(". ")
	}
	return strings.TrimSpace(b.String())
}

// TestExcludedRepos checks every line shape a spent repository is recorded in, including
// the locked sample whose origin lives in meta. Reading only the repo field is what let
// twenty-eight locked samples into a first collection run, so the meta case is pinned
// here rather than left to the lock to catch again.
func TestExcludedRepos(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	manifest := filepath.Join(dir, "manifest.jsonl")
	plain := filepath.Join(dir, "skip.txt")
	if err := os.WriteFile(manifest, []byte(
		`{"repo":"a/one","score":3}`+"\n"+`{"repo":"a/two"}`+"\n"+"\n"+`{"nope":1}`+"\n"+
			`{"id":"h001","meta":{"origin":"a/five@deadbeef00","genre":"readme"}}`+"\n"+
			`{"id":"h002","meta":{"genre":"readme"}}`+"\n"), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(plain, []byte("b/three\n\n  b/four  \n"), 0o600); err != nil {
		t.Fatalf("write skip list: %v", err)
	}

	got, err := excludedRepos(manifest, plain, filepath.Join(dir, "absent.jsonl"))
	if err != nil {
		t.Fatalf("excludedRepos: %v", err)
	}
	want := map[string]bool{
		"a/one": true, "a/two": true, "a/five": true, "b/three": true, "b/four": true,
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("excluded mismatch (-want +got):\n%s", diff)
	}
}

// TestLongformEligible pins every boundary the corpus is selected on. A passage admitted
// below the sentence floor is a passage the rhythm signals cannot read, which is the exact
// weakness this corpus exists to remove.
func TestLongformEligible(t *testing.T) {
	t.Parallel()
	s, err := sanitize.New(sanitize.DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		Name string
		In   string
		Want bool
	}{
		{Name: "long enough", In: longProse(14), Want: true},                              // Test 0.
		{Name: "too few sentences", In: longProse(longformMinSentences - 1), Want: false}, // Test 1.
		{Name: "too few words", In: strings.Repeat("Short one. ", longformMinSentences+2), // Test 2.
			Want: false},
		{Name: "too many words", In: longProse(900), Want: false},                  // Test 3.
		{Name: "not english", In: strings.Repeat("これは日本語の文章です。", 40), Want: false}, // Test 4.
		{Name: "empty", In: "", Want: false},                                       // Test 5.
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			if got := longformEligible(s, test.In); got != test.Want {
				t.Errorf("longformEligible = %v, want %v", got, test.Want)
			}
		})
	}
}

// TestOpenLongformDedupes checks that a second run sees what the first one wrote. Without
// it a repeated collection doubles a passage and every rate the corpus reports is weighted
// by how many times a sample happened to be fetched.
func TestOpenLongformDedupes(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "longform.jsonl")
	if err := os.WriteFile(path, []byte(`{"label":"ai","note":"n","text":"Already here."}`+"\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	f, seen, err := openLongform(path)
	if err != nil {
		t.Fatalf("openLongform: %v", err)
	}
	defer func() { _ = f.Close() }()
	if !seen[normalize("already   here.")] {
		t.Error("an existing passage was not seen, so a re-run would store it twice")
	}
}

// TestCollectLongformInto drives the whole collection path against a local server and
// checks the three guards together: an excluded repository is skipped, a passage too short
// for the rhythm signals is skipped, and what survives is appended rather than overwritten.
func TestCollectLongformInto(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "longform.jsonl")
	if err := os.WriteFile(out, []byte(`{"label":"ai","note":"seed","text":"Prior line."}`+"\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	manifest := filepath.Join(dir, "manifest.jsonl")
	if err := os.WriteFile(manifest, []byte(`{"repo":"spent/repo"}`+"\n"), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/search/"):
			if r.URL.Query().Get("page") != "" && r.URL.Query().Get("page") != "1" {
				_, _ = io.WriteString(w, `{"items":[]}`)
				return
			}
			_, _ = io.WriteString(w, `{"items":[
				{"full_name":"spent/repo","pushed_at":"2020-01-01T00:00:00Z","stargazers_count":60},
				{"full_name":"short/repo","pushed_at":"2020-01-01T00:00:00Z","stargazers_count":60},
				{"full_name":"good/repo","pushed_at":"2020-01-01T00:00:00Z","stargazers_count":60},
				{"full_name":"recent/repo","pushed_at":"2024-01-01T00:00:00Z","stargazers_count":60}]}`)
		case strings.Contains(r.URL.Path, "short/repo"):
			_, _ = io.WriteString(w, encodeReadme("Too short to read for rhythm.", "aaaaaaaaaabbbb"))
		case strings.Contains(r.URL.Path, "good/repo"):
			_, _ = io.WriteString(w, encodeReadme(longProse(14), "ccccccccccdddd"))
		default:
			_, _ = io.WriteString(w, encodeReadme(longProse(14), "eeeeeeeeeeffff"))
		}
	}))
	defer srv.Close()

	var log strings.Builder
	if err := testClient(srv).collectLongformInto(5, out, []string{manifest}, &log); err != nil {
		t.Fatalf("collectLongformInto: %v", err)
	}

	rows, err := readLines[LongPassage](out)
	if err != nil {
		t.Fatalf("readLines: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("corpus holds %d row(s), want the seed plus one collected passage", len(rows))
	}
	if rows[0].Text != "Prior line." {
		t.Errorf("the seed row was overwritten: %q", rows[0].Text)
	}
	if rows[1].Label != "human" || !strings.Contains(rows[1].Note, "good/repo") {
		t.Errorf("collected row = %+v, want the eligible repository labelled human", rows[1])
	}
	if strings.Contains(log.String(), "spent/repo") {
		t.Error("an excluded repository was collected, so the corpora can overlap")
	}
}

// TestGenerateLongformWith drives the machine half against a local model server and checks
// that output too short to carry a rhythm is set aside rather than stored, since a corpus
// of short machine passages would reproduce the blindness this corpus exists to fix.
func TestGenerateLongformWith(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "longform.jsonl")
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		text := longProse(14)
		if calls == 1 {
			text = "One short answer."
		}
		_, _ = io.WriteString(w, fmt.Sprintf(`{"response":%q}`, text))
	}))
	defer srv.Close()

	var log strings.Builder
	models := []string{"test-model"}
	if err := generateLongformWith(srv.Client(), srv.URL, models, nil, 1, out, &log); err != nil {
		t.Fatalf("generateLongformWith: %v", err)
	}
	rows, err := readLines[LongPassage](out)
	if err != nil {
		t.Fatalf("readLines: %v", err)
	}
	// Every combination returns the same body, so dedupe leaves exactly one row and the
	// short first answer is set aside. Both guards show up in this single count.
	if len(rows) != 1 {
		t.Fatalf("corpus holds %d row(s), want one after the short answer and the duplicates", len(rows))
	}
	if rows[0].Label != "ai" || !strings.Contains(rows[0].Note, "test-model") {
		t.Errorf("row = %+v, want an ai row naming the model", rows[0])
	}
	if !strings.Contains(log.String(), "set aside") {
		t.Errorf("log does not report what was set aside:\n%s", log.String())
	}
}

// TestLongformErrorPaths checks that a corpus run fails loudly on a source it cannot read.
// A skip list that silently reads as empty is the dangerous case: collection would carry
// on and quietly pull evaluation repositories into the development corpus.
func TestLongformErrorPaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"items":[]}`)
	}))
	defer srv.Close()

	t.Run("test 0 unreadable skip list", func(t *testing.T) {
		t.Parallel()
		if _, err := excludedRepos(dir); err == nil {
			t.Error("excludedRepos read a directory without complaining")
		}
	})
	t.Run("test 1 collection refuses an unreadable skip list", func(t *testing.T) {
		t.Parallel()
		err := testClient(srv).collectLongformInto(1, filepath.Join(dir, "a.jsonl"), []string{dir}, io.Discard)
		if err == nil {
			t.Error("collection continued past a skip list it could not read")
		}
	})
	t.Run("test 2 corpus path that cannot be opened", func(t *testing.T) {
		t.Parallel()
		if _, _, err := openLongform(dir); err == nil {
			t.Error("openLongform opened a directory as a corpus")
		}
	})
	t.Run("test 3 generation reports a model failure and carries on", func(t *testing.T) {
		t.Parallel()
		dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer dead.Close()
		var log strings.Builder
		out := filepath.Join(dir, "gen.jsonl")
		if err := generateLongformWith(dead.Client(), dead.URL, []string{"m"}, nil, 1, out, &log); err != nil {
			t.Fatalf("generateLongformWith: %v", err)
		}
		if !strings.Contains(log.String(), "appended 0 machine passage(s)") {
			t.Errorf("a dead model server did not report an empty run:\n%s", log.String())
		}
	})
}

// TestGenerateLongformGenreFilter checks that naming genres restricts the run to them.
// Filling in a genre added after a collection needs this, and a filter that silently
// walked every genre would spend an hour regenerating what the corpus already holds.
func TestGenerateLongformGenreFilter(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "longform.jsonl")
	var prompts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Prompt string `json:"prompt"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		prompts = append(prompts, body.Prompt)
		_, _ = io.WriteString(w, fmt.Sprintf(`{"response":%q}`, longProse(14)))
	}))
	defer srv.Close()

	err := generateLongformWith(srv.Client(), srv.URL, []string{"m"}, []string{"readme"}, 1, out, io.Discard)
	if err != nil {
		t.Fatalf("generateLongformWith: %v", err)
	}
	// One genre against three prompt styles is three calls, and no other genre's task
	// should appear in any of them.
	if len(prompts) != len(genStyles) {
		t.Fatalf("made %d call(s), want %d: one per style of the single named genre",
			len(prompts), len(genStyles))
	}
	for _, p := range prompts {
		if !strings.Contains(p, "Write the README") {
			t.Errorf("a genre outside the filter was generated: %q", clipPrompt(p))
		}
	}
}

// clipPrompt shortens a prompt for a failure message.
func clipPrompt(s string) string {
	if len(s) <= 60 {
		return s
	}
	return s[:60] + "..."
}
