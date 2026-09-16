package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// sampleSet returns n placeholder samples for the ordering and resume tests.
func sampleSet(n int) []Sample {
	out := make([]Sample, 0, n)
	for i := range n {
		out = append(out, Sample{ID: fmt.Sprintf("s%02d", i), Source: "human", Text: "text"})
	}
	return out
}

// TestRateShuffleVisitsEverySample checks the property the blinding rests on: every
// sample is presented exactly once, and a different seed gives a different order. The
// collection order groups all the machine samples together, so presenting in it would
// hand the rater the answer.
func TestRateShuffleVisitsEverySample(t *testing.T) {
	t.Parallel()
	for testNum, n := range []int{1, 2, 7, 24, 100} {
		t.Run(fmt.Sprintf("test %d size %d", testNum, n), func(t *testing.T) {
			t.Parallel()
			got := rateShuffle(sampleSet(n), 5)
			if len(got) != n {
				t.Fatalf("got %d samples, want %d", len(got), n)
			}
			seen := map[string]int{}
			for _, s := range got {
				seen[s.ID]++
			}
			if len(seen) != n {
				t.Errorf("presented %d distinct samples, want %d", len(seen), n)
			}
			for id, count := range seen {
				if count != 1 {
					t.Errorf("sample %s presented %d times, want once", id, count)
				}
			}
		})
	}
}

// TestRateShuffleDiffersBySeed checks that two raters do not see the same order, which is
// what keeps one rater's fatigue from landing on the same samples as another's.
func TestRateShuffleDiffersBySeed(t *testing.T) {
	t.Parallel()
	a := rateShuffle(sampleSet(24), 1)
	b := rateShuffle(sampleSet(24), 9)
	same := true
	for i := range a {
		if a[i].ID != b[i].ID {
			same = false
			break
		}
	}
	if same {
		t.Errorf("two seeds produced the same order")
	}
}

// TestRated checks that a resumed session skips what this rater already answered while
// leaving another rater's answers alone.
func TestRated(t *testing.T) {
	t.Parallel()
	got := rated([]Rating{
		{Sample: "a", Rater: "me", Machine: 3},
		{Sample: "b", Rater: "you", Machine: 4},
		{Sample: "c", Rater: "me", Machine: 5},
	}, "me")
	want := map[string]bool{"a": true, "c": true}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("rated mismatch (-want +got):\n%s", diff)
	}
}

// TestRunRate checks a whole session: answers are recorded, an out-of-range answer is
// refused rather than stored, quitting stops the walk, and the label never reaches the
// screen, since a rater who can see it is not blind.
func TestRunRate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	samples := dir + "/samples.jsonl"
	ratings := dir + "/ratings.jsonl"
	body := strings.Join([]string{
		`{"id":"h001","source":"human","rules":"v1","text":"A person wrote this one."}`,
		`{"id":"a001","source":"ai","rules":"v1","text":"A machine wrote this one."}`,
		`{"id":"a002","source":"ai","rules":"v1","text":"And this one too."}`,
	}, "\n")
	if err := os.WriteFile(samples, []byte(body+"\n"), 0o600); err != nil {
		t.Fatalf("write samples: %v", err)
	}

	var out strings.Builder
	// A good answer, an answer out of range, then quit.
	in := strings.NewReader("4\n99\nq\n")
	if err := runRate(samples, ratings, "rater-one", 2, in, &out); err != nil {
		t.Fatalf("runRate: %v", err)
	}
	got, err := readLines[Rating](ratings)
	if err != nil {
		t.Fatalf("readLines: %v", err)
	}
	if len(got) != 1 || got[0].Rater != "rater-one" || got[0].Machine != 4 {
		t.Errorf("ratings = %+v, want one answer of 4 from rater-one", got)
	}
	for _, leak := range []string{`"ai"`, `"human"`, "source", "a001", "h001"} {
		if strings.Contains(out.String(), leak) {
			t.Errorf("the session showed %q, which tells the rater the answer:\n%s", leak, out.String())
		}
	}
}

// TestRunRateResumes checks that a second session skips what the first one answered.
func TestRunRateResumes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	samples := dir + "/samples.jsonl"
	ratings := dir + "/ratings.jsonl"
	body := `{"id":"h001","source":"human","rules":"v1","text":"One."}
{"id":"h002","source":"human","rules":"v1","text":"Two."}`
	if err := os.WriteFile(samples, []byte(body+"\n"), 0o600); err != nil {
		t.Fatalf("write samples: %v", err)
	}
	var first strings.Builder
	if err := runRate(samples, ratings, "me", 1, strings.NewReader("3\nq\n"), &first); err != nil {
		t.Fatalf("first session: %v", err)
	}
	var second strings.Builder
	if err := runRate(samples, ratings, "me", 1, strings.NewReader("q\n"), &second); err != nil {
		t.Fatalf("second session: %v", err)
	}
	if !strings.Contains(second.String(), "1 left") {
		t.Errorf("resumed session did not skip the answered sample:\n%s", second.String())
	}
}

// TestRunRateNeedsRater checks that an answer is always attributable.
func TestRunRateNeedsRater(t *testing.T) {
	t.Parallel()
	if err := runRate("x", "y", "  ", 1, strings.NewReader(""), &strings.Builder{}); err == nil {
		t.Errorf("err = nil, want a refusal without a rater id")
	}
}

// TestGenerateAIFrom drives the generator against a local server: a reply inside the band
// is kept with its provenance, and one outside it is discarded rather than trimmed,
// because trimming would edit the model's prose and the sample is meant to be what it
// actually wrote.
func TestGenerateAIFrom(t *testing.T) {
	t.Parallel()
	long := strings.TrimSpace(strings.Repeat("the crew set a benchmark on the ridge that morning ", 12))
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		text := long
		if calls%2 == 0 {
			text = "too short"
		}
		_, _ = io.WriteString(w, fmt.Sprintf(`{"response":%q}`, text))
	}))
	defer srv.Close()

	path := t.TempDir() + "/samples.jsonl"
	var out strings.Builder
	if err := generateAIFrom(srv.URL, []string{"testmodel"}, 1, path, "v9.9.9", &out); err != nil {
		t.Fatalf("generateAIFrom: %v", err)
	}
	got, err := readLines[Sample](path)
	if err != nil {
		t.Fatalf("readLines: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("no samples written")
	}
	if !strings.Contains(out.String(), "discarded") {
		t.Errorf("the run did not report what it discarded:\n%s", out.String())
	}
	genres := map[string]bool{}
	for i, s := range got {
		if s.Source != "ai" {
			t.Errorf("sample %s source = %q, want ai", s.ID, s.Source)
		}
		if s.Rules != "v9.9.9" {
			t.Errorf("sample %s rules = %q, want the frozen tag", s.ID, s.Rules)
		}
		if s.Meta["model"] != "testmodel" || s.Meta["prompt"] == "" || s.Meta["genre"] == "" {
			t.Errorf("sample %s meta = %v, want model, prompt, and genre recorded", s.ID, s.Meta)
		}
		if n := len(strings.Fields(s.Text)); n < genMinWords || n > genMaxWords {
			t.Errorf("sample %s is %d words, outside the band", s.ID, n)
		}
		if want := fmt.Sprintf("a%03d", i+1); s.ID != want {
			t.Errorf("sample id = %q, want %q", s.ID, want)
		}
		genres[s.Meta["genre"]] = true
	}
	if len(genres) < 2 {
		t.Errorf("genres covered = %d, want the run to spread across them", len(genres))
	}
}

// TestGenerateAIFromContinuesIDs checks that a second run does not collide with the ids
// the first one wrote, since the corpus is appended to rather than replaced.
func TestGenerateAIFromContinuesIDs(t *testing.T) {
	t.Parallel()
	long := strings.TrimSpace(strings.Repeat("the crew set a benchmark on the ridge that morning ", 12))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, fmt.Sprintf(`{"response":%q}`, long))
	}))
	defer srv.Close()

	path := t.TempDir() + "/samples.jsonl"
	for range 2 {
		if err := generateAIFrom(srv.URL, []string{"m"}, 1, path, "v1", io.Discard); err != nil {
			t.Fatalf("generateAIFrom: %v", err)
		}
	}
	got, err := readLines[Sample](path)
	if err != nil {
		t.Fatalf("readLines: %v", err)
	}
	seen := map[string]bool{}
	for _, s := range got {
		if seen[s.ID] {
			t.Errorf("duplicate id %s across runs", s.ID)
		}
		seen[s.ID] = true
	}
}

// TestProseOnly checks the filter the human half depends on. A sample still carrying link
// syntax, a code span, or a list fragment is distinguishable from a generated one on
// markup alone, which hands the rater the answer as surely as the label would.
func TestProseOnly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name     string
		In       string
		WantHas  string
		WantGone []string
	}{{ // Test 0: A link keeps its words and loses its destination.
		Name: "inline link", WantHas: "uses the Raft library for consensus",
		In:       "This project uses the [Raft](https://example.com/raft) library for consensus here.",
		WantGone: []string{"](", "http"},
	}, { // Test 1: A code span is markup, not writing.
		Name: "code span", WantHas: "",
		In:       "Send the payload as `{\"name\": \"x\"}` to the server endpoint now.",
		WantGone: []string{"`", "{"},
	}, { // Test 2: A fenced block never reaches the sample.
		Name: "fence", WantHas: "The prose survives the fence around it",
		In:       "```go\nfunc main() {}\n```\nThe prose survives the fence around it.",
		WantGone: []string{"func main"},
	}, { // Test 3: A numbered list is a list whatever it starts with.
		Name: "numbered list", WantHas: "",
		In:       "1. First item here\n2) Second item there",
		WantGone: []string{"First item", "Second item"},
	}, { // Test 4: Headings, badges, tables, and bullets are structure.
		Name: "structure", WantHas: "",
		In:       "# Title\n![badge](x)\n| a | b |\n- bullet point here\n> quoted line here",
		WantGone: []string{"Title", "badge", "bullet", "quoted"},
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			got := proseOnly(test.In)
			if test.WantHas != "" && !strings.Contains(got, test.WantHas) {
				t.Errorf("proseOnly dropped the prose: got %q, want it to contain %q", got, test.WantHas)
			}
			for _, gone := range test.WantGone {
				if strings.Contains(got, gone) {
					t.Errorf("proseOnly left %q in: %q", gone, got)
				}
			}
		})
	}
}

// TestLoadExcluded checks the lock guard: repositories already spent on the false
// positive measurement must not reappear in the corpus that has to stay untouched.
func TestLoadExcluded(t *testing.T) {
	t.Parallel()
	path := t.TempDir() + "/used.txt"
	if err := os.WriteFile(path, []byte("owner/one\n\nowner/two\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := loadExcluded(path)
	if err != nil {
		t.Fatalf("loadExcluded: %v", err)
	}
	if !got["owner/one"] || !got["owner/two"] || len(got) != 2 {
		t.Errorf("excluded = %v, want the two named repositories", got)
	}
	empty, err := loadExcluded("")
	if err != nil || len(empty) != 0 {
		t.Errorf("loadExcluded(\"\") = %v, %v, want an empty set and no error", empty, err)
	}
}

// TestCollectHumanInto drives the human collection against a local server. The guards
// that matter are the lock and the date: a repository already spent on the false positive
// measurement must not reappear, and a repository pushed after the cutoff must not either.
func TestCollectHumanInto(t *testing.T) {
	t.Parallel()
	prose := strings.TrimSpace(strings.Repeat("The scheduler keeps a queue of pending jobs and drains it in order. ", 14))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/search/") {
			_, _ = io.WriteString(w, `{"items":[
				{"full_name":"owner/used","pushed_at":"2020-01-01T00:00:00Z"},
				{"full_name":"owner/fresh","pushed_at":"2020-01-01T00:00:00Z"},
				{"full_name":"owner/toonew","pushed_at":"2024-01-01T00:00:00Z"}]}`)
			return
		}
		_, _ = io.WriteString(w, fmt.Sprintf(`{"content":%q,"encoding":"base64","sha":"abcdef1234567890"}`,
			base64.StdEncoding.EncodeToString([]byte(prose))))
	}))
	defer srv.Close()

	dir := t.TempDir()
	exclude := dir + "/used.txt"
	if err := os.WriteFile(exclude, []byte("owner/used\n"), 0o600); err != nil {
		t.Fatalf("write exclude: %v", err)
	}
	samples := dir + "/samples.jsonl"
	if err := testClient(srv).collectHumanInto(5, samples, exclude, "v1.2.3", io.Discard); err != nil {
		t.Fatalf("collectHumanInto: %v", err)
	}
	got, err := readLines[Sample](samples)
	if err != nil {
		t.Fatalf("readLines: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("no human samples written")
	}
	for _, s := range got {
		if s.Source != "human" {
			t.Errorf("sample %s source = %q, want human", s.ID, s.Source)
		}
		if s.Meta["genre"] != "readme" || s.Meta["origin"] == "" {
			t.Errorf("sample %s meta = %v, want genre and origin recorded", s.ID, s.Meta)
		}
		if strings.Contains(s.Meta["origin"], "owner/used") {
			t.Errorf("a repository from the false positive run reached the locked corpus: %s", s.Meta["origin"])
		}
		if strings.Contains(s.Meta["origin"], "owner/toonew") {
			t.Errorf("a repository pushed after the cutoff reached the corpus: %s", s.Meta["origin"])
		}
		if s.Rules != "v1.2.3" {
			t.Errorf("sample %s rules = %q, want the frozen tag", s.ID, s.Rules)
		}
	}
}
