package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/dcadolph/slop-chop/rewrite"
	"github.com/dcadolph/slop-chop/sanitize"
)

// stubLearn swaps the learn pass for a fake that returns reply, restoring it after the test.
func stubLearn(t *testing.T, reply string) {
	t.Helper()
	old := learnPass
	learnPass = func(_ context.Context, _ rewrite.Completer, _ string) (string, error) {
		return reply, nil
	}
	t.Cleanup(func() { learnPass = old })
}

// TestVoiceLearn checks that learn derives tone notes, merges them into an existing voice
// without duplicates, and keeps the other lists intact.
func TestVoiceLearn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "voice.json")
	seed := `{"keep":["gnarly"],"tone":["dry humor"]}`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	stubLearn(t, "Here you go:\n[\"short, blunt sentences\", \"Dry Humor\", \"opens with the point\"]")

	_, stderr, err := runCLI(t, []string{"voice", "learn", "--voice", path}, "sample writing here")
	if err != nil {
		t.Fatalf("voice learn: %v", err)
	}
	if !strings.Contains(stderr, "learned 3 tone note(s)") {
		t.Errorf("stderr = %q, want a learned count", stderr)
	}

	v, err := sanitize.LoadVoiceFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// "Dry Humor" collapses into the seeded "dry humor"; the other two append in order.
	wantTone := []string{"dry humor", "short, blunt sentences", "opens with the point"}
	if diff := cmp.Diff(wantTone, v.Tone); diff != "" {
		t.Errorf("tone mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"gnarly"}, v.Keep); diff != "" {
		t.Errorf("keep clobbered (-want +got):\n%s", diff)
	}
}

// TestVoiceLearnCreatesFile checks that learn writes a fresh voice file when none exists.
func TestVoiceLearnCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh", "voice.json")
	stubLearn(t, `["contractions everywhere"]`)

	if _, _, err := runCLI(t, []string{"voice", "learn", "--voice", path}, "sample"); err != nil {
		t.Fatalf("voice learn: %v", err)
	}
	v, err := sanitize.LoadVoiceFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"contractions everywhere"}, v.Tone); diff != "" {
		t.Errorf("tone mismatch (-want +got):\n%s", diff)
	}
}

// TestVoiceLearnErrors checks the error paths: no samples, and a reply with no array.
func TestVoiceLearnErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "voice.json")

	// Test 0: empty stdin is an error.
	stubLearn(t, `["x"]`)
	if _, _, err := runCLI(t, []string{"voice", "learn", "--voice", path}, "   "); err == nil ||
		!strings.Contains(err.Error(), "no samples") {
		t.Errorf("empty stdin: err = %v, want no-samples", err)
	}

	// Test 1: a reply with no JSON array is an error.
	stubLearn(t, "I could not derive a voice.")
	if _, _, err := runCLI(t, []string{"voice", "learn", "--voice", path}, "sample"); err == nil ||
		!strings.Contains(err.Error(), "no JSON array") {
		t.Errorf("bad reply: err = %v, want no-JSON-array", err)
	}
}

// TestParseToneNotes checks array extraction from noisy replies.
func TestParseToneNotes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		WantNotes []string
		In        string
		WantErr   bool
	}{{ // Test 0: a bare array parses.
		In: `["a", "b"]`, WantNotes: []string{"a", "b"},
	}, { // Test 1: prose and fences around the array are tolerated.
		In: "Sure thing:\n```json\n[\"a\"]\n```", WantNotes: []string{"a"},
	}, { // Test 2: blank entries are dropped.
		In: `["a", "  ", ""]`, WantNotes: []string{"a"},
	}, { // Test 3: no array is an error.
		In: "nothing here", WantErr: true,
	}, { // Test 4: an array of only blanks is an error.
		In: `["", " "]`, WantErr: true,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got, err := parseToneNotes(test.In)
			if test.WantErr {
				if err == nil {
					t.Fatalf("err = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseToneNotes: %v", err)
			}
			if diff := cmp.Diff(test.WantNotes, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestMergeToneNotes checks that merging dedupes case-insensitively and keeps order.
func TestMergeToneNotes(t *testing.T) {
	t.Parallel()
	got := mergeToneNotes([]string{"Dry humor", "short"}, []string{"dry humor", "new note", "short"})
	want := []string{"Dry humor", "short", "new note"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

// TestVoiceToneReachesRewrite checks the whole wire: tone lines in a voice file arrive at
// the rewrite pass when fix --rewrite runs.
func TestVoiceToneReachesRewrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "voice.json")
	if err := os.WriteFile(path, []byte(`{"tone":["dry humor","short sentences"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotTone []string
	old := rewritePass
	rewritePass = func(_ context.Context, _ rewrite.Completer, tone []string, text string,
		_ ...string) (string, error) {
		gotTone = tone
		return text, nil
	}
	t.Cleanup(func() { rewritePass = old })

	if _, _, err := runCLI(t,
		[]string{"fix", "--rewrite", "--verify=false", "--voice", path}, "a plain line"); err != nil {
		t.Fatalf("fix --rewrite: %v", err)
	}
	for _, want := range []string{"dry humor", "short sentences"} {
		if !slices.Contains(gotTone, want) {
			t.Errorf("tone = %v, want it to contain %q", gotTone, want)
		}
	}
}

// TestVoiceDiff checks that voice diff groups a draft's changes into the three readings
// and prints a voice fragment holding only the ones that would add a rule.
func TestVoiceDiff(t *testing.T) {
	dir := t.TempDir()
	draft := writeTemp(t, dir, "draft.md",
		"The robust plan is comprehensive and we commence the rollout.\n")
	final := writeTemp(t, dir, "final.md",
		"The solid plan is comprehensive and we start the rollout.\n")

	stdout, _, err := runCLI(t, []string{"voice", "diff", draft, final}, "")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	for _, want := range []string{
		"keep:", "comprehensive",
		"prefer:", `"commence" -> "start"`,
		"confirms:", "word:robust",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q\n%s", want, stdout)
		}
	}
	// The confirmed cut already has a rule, so it must not be proposed again.
	if strings.Contains(stdout, `"robust":`) {
		t.Errorf("suggested voice proposed a rule that already exists\n%s", stdout)
	}
}

// TestVoiceDiffJSON checks the machine shape carries both the candidates and the voice
// they would add.
func TestVoiceDiffJSON(t *testing.T) {
	dir := t.TempDir()
	draft := writeTemp(t, dir, "draft.md", "we commence the rollout now\n")
	final := writeTemp(t, dir, "final.md", "we start the rollout now\n")

	stdout, _, err := runCLI(t, []string{"voice", "diff", "--json", draft, final}, "")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	var report struct {
		Candidates []sanitize.Candidate `json:"candidates"`
		Suggested  sanitize.Voice       `json:"suggested"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode: %v (%s)", err, stdout)
	}
	if len(report.Candidates) == 0 {
		t.Fatalf("no candidates in %s", stdout)
	}
	if got := report.Suggested.Prefer["commence"]; got != "start" {
		t.Errorf("suggested prefer[commence] = %q, want start", got)
	}
}

// TestVoiceDiffOnlyFacts checks that two versions differing only in a number propose
// nothing, since a corrected fact says nothing about voice.
func TestVoiceDiffOnlyFacts(t *testing.T) {
	dir := t.TempDir()
	draft := writeTemp(t, dir, "draft.md", "the retry waits 5s before giving up\n")
	final := writeTemp(t, dir, "final.md", "the retry waits 30s before giving up\n")

	stdout, _, err := runCLI(t, []string{"voice", "diff", draft, final}, "")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !strings.Contains(stdout, "no candidates") {
		t.Errorf("stdout = %q, want no candidates for a fact-only change", stdout)
	}
}
