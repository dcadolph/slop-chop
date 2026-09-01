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
		WantResult: []EditPair{{Was: "In summary", Now: ""}},
	}, {
		Name:       "restored capital after a cut is not a change",
		Draft:      "In summary, the plan works",
		Final:      "The plan works",
		WantResult: []EditPair{{Was: "In summary", Now: ""}},
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
	want := []EditPair{{Was: "robust", Now: "solid"}, {Was: "utilizes", Now: "uses"}}
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
		{Was: "robust", Now: "solid"},
		{Was: "It is important to note that", Now: ""},
	}
	if diff := cmp.Diff(want, EditPairs(draft, final), cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("EditPairs mismatch (-want +got):\n%s", diff)
	}
}

// TestEditPairsPhantoms pins the pairing pathologies the correctness audit found: a
// moved word is not a cut, a restructure absorbing a sentence tail fabricates nothing,
// dotted abbreviations do not split pairs, and an unsegmented script cannot propose the
// whole document as one rule.
func TestEditPairsPhantoms(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name       string
		Draft      string
		Final      string
		WantResult []EditPair
	}{{
		Name:       "moved word is not a cut",
		Draft:      "He quickly ran home before the storm hit.",
		Final:      "He ran quickly home before the storm hit.",
		WantResult: nil,
	}, {
		Name:       "reorder inside a longer run is not a cut",
		Draft:      "alpha bravo charlie alpha",
		Final:      "alpha charlie bravo alpha",
		WantResult: nil,
	}, {
		Name:       "rewrite absorbing a sentence tail fabricates no cut",
		Draft:      "Start alpha beta gamma delta. epsilon zeta eta End",
		Final:      "Start good End",
		WantResult: []EditPair{{Was: "alpha beta gamma delta", Now: "good"}},
	}, {
		Name:       "dotted abbreviation does not split a pair",
		Draft:      "Start used tools e.g. hammers End",
		Final:      "Start picked mallets End",
		WantResult: []EditPair{{Was: "used tools e.g. hammers", Now: "picked mallets"}},
	}, {
		Name:       "initials do not split a pair",
		Draft:      "Start the U.S. office soon End",
		Final:      "Start the branch quickly End",
		WantResult: []EditPair{{Was: "U.S. office soon", Now: "branch quickly"}},
	}, {
		Name:       "unsegmented script yields nothing",
		Draft:      "私は犬が好きですと彼は言いましたがそれは長い話でありまして続きます",
		Final:      "私は猫が好きですと彼は言いましたがそれは長い話でありまして続きます",
		WantResult: nil,
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

// TestEditPairsParagraphFallback checks that a long document whose whole-text table
// would blow the cell cap still yields its pairs through paragraph alignment.
func TestEditPairsParagraphFallback(t *testing.T) {
	t.Parallel()
	var d, f strings.Builder
	for i := 0; i < 80; i++ {
		for j := 0; j < 20; j++ {
			d.WriteString("the team worked carefully on every part of rollout number " +
				strings.Repeat("x", 1+i%3) + " and shipped it very reliably today without regressions ")
			f.WriteString("the team worked with care on every part of rollout number " +
				strings.Repeat("x", 1+i%3) + " and shipped it without fail without regressions ")
		}
		d.WriteString("\n\n")
		f.WriteString("\n\n")
	}
	pairs := EditPairs(d.String(), f.String())
	if len(pairs) == 0 {
		t.Fatal("paragraph fallback produced no pairs for a heavily edited long document")
	}
	seen := map[string]bool{}
	for _, p := range pairs {
		seen[p.Was+"->"+p.Now] = true
	}
	if !seen["carefully->with care"] || !seen["very reliably today->without fail"] {
		t.Errorf("expected the two repeated edits in %v", pairs[:min(4, len(pairs))])
	}
}

// TestCandidatesStopWordGuard checks that restructure residue on common words is never
// proposed as a rule.
func TestCandidatesStopWordGuard(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	draft := "We utilized a comprehensive rollout plan and the cutover was seamless for users."
	final := "We rolled it out carefully and the cutover went cleanly for users."
	for _, c := range s.Candidates(draft, final) {
		if c.Kind != CandidateNew {
			continue
		}
		w := strings.ToLower(c.Pair.Was)
		if pairStopWords[w] {
			t.Errorf("stop word %q proposed as a rule: %+v", w, c)
		}
	}
}

// TestCandidatesMarkdownGuard checks that markdown structure never becomes a voice
// proposal.
func TestCandidatesMarkdownGuard(t *testing.T) {
	t.Parallel()
	s, err := New(DefaultProfile())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	draft := "intro line\n\n- **Resilience**: caches absorb spikes.\n- **Scalability**: caches scale horizontally.\n"
	final := "intro line\n\ncaches absorb spikes and scale horizontally.\n"
	for _, c := range s.Candidates(draft, final) {
		if c.Kind == CandidateNew && strings.Contains(c.Pair.Was, "*") {
			t.Errorf("markdown fragment proposed as a rule: %+v", c)
		}
	}
}
