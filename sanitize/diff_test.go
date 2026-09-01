package sanitize

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// TestEditPairs checks that a draft and its edited version yield the word choices the
// writer made and nothing else: facts, additions, and restructured sentences are left
// out, since none of them says anything about preferred wording.
func TestEditPairs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name       string
		Draft      string
		Final      string
		WantResult []EditPair
	}{{
		Name:       "one word swapped",
		Draft:      "we utilize the parser every day",
		Final:      "we use the parser every day",
		WantResult: []EditPair{{Was: "utilize", Now: "use"}},
	}, {
		Name:       "two separate swaps group apart",
		Draft:      "we utilize robust tools here",
		Final:      "we use solid tools here",
		WantResult: []EditPair{{Was: "utilize", Now: "use"}, {Was: "robust", Now: "solid"}},
	}, {
		Name:       "cut opener is a pair with an empty replacement",
		Draft:      "In summary, the plan works",
		Final:      "the plan works",
		WantResult: []EditPair{{Was: "In summary,", Now: ""}},
	}, {
		Name:       "restored capital after a cut is not a change",
		Draft:      "In summary, the plan works",
		Final:      "The plan works",
		WantResult: []EditPair{{Was: "In summary,", Now: ""}},
	}, {
		Name:       "a changed number is a fact, not a preference",
		Draft:      "the retry waits 5s before giving up",
		Final:      "the retry waits 30s before giving up",
		WantResult: nil,
	}, {
		Name:       "a swap next to an unchanged number survives",
		Draft:      "the retry utilizes 5s of backoff",
		Final:      "the retry uses 5s of backoff",
		WantResult: []EditPair{{Was: "utilizes", Now: "uses"}},
	}, {
		Name:       "a dropped URL is a fact change",
		Draft:      "see https://example.com/a for the guide",
		Final:      "see the guide",
		WantResult: nil,
	}, {
		Name:       "pure insertion is the writer's own content",
		Draft:      "the plan works",
		Final:      "the plan works well in practice",
		WantResult: nil,
	}, {
		// A rewritten sentence yields its parts rather than nothing. The cut opener is
		// the signal worth having; the rest is why a rule has to clear a frequency bar
		// before it graduates, which is the caller's job and not this primitive's.
		Name:  "a restructured sentence yields its parts",
		Draft: "It is important to note that the parser will handle every one of these cases",
		Final: "The parser handles all of them",
		WantResult: []EditPair{
			{Was: "It is important to note that", Now: ""},
			{Was: "will handle every one", Now: "handles all"},
			{Was: "these cases", Now: "them"},
		},
	}, {
		Name:       "identical text yields nothing",
		Draft:      "the plan works",
		Final:      "the plan works",
		WantResult: nil,
	}, {
		Name:       "empty inputs yield nothing",
		Draft:      "",
		Final:      "",
		WantResult: nil,
	}, {
		Name:       "a multi-word phrase swap stays one pair",
		Draft:      "we need to leverage the power of caching",
		Final:      "we need to use caching",
		WantResult: []EditPair{{Was: "leverage the power of", Now: "use"}},
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			got := EditPairs(test.Draft, test.Final)
			if diff := cmp.Diff(test.WantResult, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("EditPairs mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestEditPairsAcrossLines checks that a diff spanning several lines still pairs the
// change with its own wording rather than merging unrelated edits.
func TestEditPairsAcrossLines(t *testing.T) {
	t.Parallel()
	draft := "The build is robust.\n\nIt utilizes a cache to stay fast.\n"
	final := "The build is solid.\n\nIt uses a cache to stay fast.\n"
	want := []EditPair{{Was: "robust.", Now: "solid."}, {Was: "utilizes", Now: "uses"}}
	if diff := cmp.Diff(want, EditPairs(draft, final), cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("EditPairs mismatch (-want +got):\n%s", diff)
	}
}

// TestEditPairsHugeInputBails checks that two unrelated documents past the table cap
// report nothing rather than allocating without bound.
func TestEditPairsHugeInputBails(t *testing.T) {
	t.Parallel()
	var a, b []byte
	for i := 0; i < 2000; i++ {
		a = append(a, []byte("alpha ")...)
		b = append(b, []byte("beta ")...)
	}
	if got := EditPairs(string(a), string(b)); got != nil {
		t.Errorf("EditPairs on a huge unrelated pair = %d pairs, want none", len(got))
	}
}

// TestCandidates checks the three readings of a change: cutting a flagged tell confirms
// the rule that flags it, changing something no rule caught is where a profile grows,
// and a tell the writer read and shipped is reported against the rules.
func TestCandidates(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// The writer cut a flagged buzzword, swapped a word nothing flags, and shipped
	// "comprehensive" untouched.
	draft := "The robust plan is comprehensive and we commence the work today."
	final := "The solid plan is comprehensive and we start the work today."

	byKind := map[CandidateKind][]Candidate{}
	for _, c := range s.Candidates(draft, final) {
		byKind[c.Kind] = append(byKind[c.Kind], c)
	}

	// Test 0: cutting "robust" confirms the rule that already flags it.
	if got := byKind[CandidateConfirms]; len(got) != 1 || got[0].Pair.Was != "robust" {
		t.Errorf("confirms = %+v, want one for robust", got)
	} else if got[0].Rule != "word:robust" {
		t.Errorf("confirms rule = %q, want word:robust", got[0].Rule)
	}

	// Test 1: "commence" to "start" is a swap no rule caught, so it is a new candidate.
	found := false
	for _, c := range byKind[CandidateNew] {
		if c.Pair.Was == "commence" && c.Pair.Now == "start" {
			found = true
		}
	}
	if !found {
		t.Errorf("new = %+v, want the commence to start swap", byKind[CandidateNew])
	}

	// Test 2: "comprehensive" survived the edit pass, so it is a keep candidate.
	if got := byKind[CandidateKeep]; len(got) != 1 || !strings.EqualFold(got[0].Match, "comprehensive") {
		t.Errorf("keep = %+v, want one for comprehensive", got)
	}
}

// TestCandidatesNoEdits checks that an unedited draft yields nothing, since a document
// nobody changed says nothing about how its writer writes.
func TestCandidatesNoEdits(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	text := "The robust plan is comprehensive."
	if got := s.Candidates(text, text); got != nil {
		t.Errorf("Candidates on an unedited draft = %+v, want none", got)
	}
}

// TestEditPairsSentenceBoundary checks that a run of changes crossing a sentence end is
// cut at that boundary. Merged, the end of one sentence and the cut opener of the next
// become a single pair describing neither.
func TestEditPairsSentenceBoundary(t *testing.T) {
	t.Parallel()
	draft := "The pipeline is robust. It is important to note that we ship on Friday."
	final := "The pipeline is solid. We ship on Friday."
	want := []EditPair{
		{Was: "robust.", Now: "solid."},
		{Was: "It is important to note that", Now: ""},
	}
	if diff := cmp.Diff(want, EditPairs(draft, final), cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("EditPairs mismatch (-want +got):\n%s", diff)
	}
}
