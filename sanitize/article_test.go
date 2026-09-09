package sanitize

import (
	"fmt"
	"testing"
)

// TestStartsWithVowelSoundAllCaps pins the split between an initialism a reader spells and
// an acronym a reader says. The letter-name rule is right only for the first kind: the R of
// RFC is said "ar", so it takes "an". Applying it to the second kind produced "an README"
// and "an REST API", which is visibly wrong in the one place a writing tool cannot afford
// to be, its own correction. Both directions are pinned here, since a fix that stopped
// saying "an README" by dropping the letter-name rule would start saying "a RFC".
func TestStartsWithVowelSoundAllCaps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Word string
		Want bool
	}{{ // Test 0: README is read "read-me", so it opens on a consonant.
		Word: "README", Want: false,
	}, { // Test 1: REST is read "rest", not R-E-S-T.
		Word: "REST", Want: false,
	}, { // Test 2: RFC is spelled out, and "ar" opens on a vowel.
		Word: "RFC", Want: true,
	}, { // Test 3: HTML is spelled out, and "aitch" opens on a vowel.
		Word: "HTML", Want: true,
	}, { // Test 4: SQL is spelled out by the rule, and "ess" opens on a vowel.
		Word: "SQL", Want: true,
	}, { // Test 5: URL is spelled out but "you" opens on a consonant.
		Word: "URL", Want: false,
	}, { // Test 6: API is spelled out, and "ay" opens on a vowel.
		Word: "API", Want: true,
	}, { // Test 7: NASA is said as a word opening on a consonant.
		Word: "NASA", Want: false,
	}, { // Test 8: LASER is a word, whatever its letters are named.
		Word: "LASER", Want: false,
	}, { // Test 9: trailing punctuation must not defeat the lookup.
		Word: "README's", Want: false,
	}, { // Test 10: an ordinary lowercase word is unaffected.
		Word: "hour", Want: true,
	}, { // Test 11: and so is an ordinary consonant word.
		Word: "readme", Want: false,
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Word), func(t *testing.T) {
			t.Parallel()
			if got := startsWithVowelSound(test.Word); got != test.Want {
				article := map[bool]string{true: "an", false: "a"}
				t.Errorf("startsWithVowelSound(%q) = %v, want %v (so %q %s)",
					test.Word, got, test.Want, article[test.Want], test.Word)
			}
		})
	}
}

// TestArticleNeedsFixAllCaps checks the rule end to end, so a correct article in real prose
// is left alone rather than reported. A tool that flags correct writing costs its user more
// than one it stays quiet on.
func TestArticleNeedsFixAllCaps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		In   string
		Want bool
	}{{ // Test 0: the case that started this, now quiet.
		In: "a README is untrusted input", Want: false,
	}, { // Test 1: and its opposite, still caught.
		In: "an README is untrusted input", Want: true,
	}, { // Test 2: a spelled initialism still takes an.
		In: "a RFC describes it", Want: true,
	}, { // Test 3: and is quiet when already correct.
		In: "an RFC describes it", Want: false,
	}, { // Test 4: a word-acronym in ordinary prose.
		In: "a REST API answers", Want: false,
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			if got := articleNeedsFix(test.In, 0, len(test.In)); got != test.Want {
				t.Errorf("articleNeedsFix(%q) = %v, want %v", test.In, got, test.Want)
			}
		})
	}
}

// TestContestedAcronymsStaySilent checks the words careful writers pronounce two ways.
// SQL is "sequel" in one house and "ess-cue-ell" in the next, which is why Oracle writes
// "a SQL statement" and PostgreSQL writes "an SQL statement". Both are right, so the rule
// must not report either. Picking one would correct a writer's house style, which is a
// worse failure than staying quiet. FAQ is not on the list: this project already picked
// "an FAQ" and pins it, so that choice stands.
func TestContestedAcronymsStaySilent(t *testing.T) {
	t.Parallel()

	for _, in := range []string{
		"a SQL query", "an SQL query",
		"a SQL's plan", "an SQL's plan",
	} {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			if articleNeedsFix(in, 0, len(in)) {
				t.Errorf("articleNeedsFix(%q) fired; both readings are defensible", in)
			}
		})
	}

	// The silence is scoped to the contested list, so an ordinary initialism is
	// still corrected.
	if !articleNeedsFix("a RFC describes it", 0, len("a RFC describes it")) {
		t.Error("a RFC should still be corrected to an RFC")
	}
}
