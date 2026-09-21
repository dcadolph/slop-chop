package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dcadolph/slop-chop/sanitize"
)

// The rule exemplar corpus in sanitize/testdata/corpus.jsonl holds the smallest text that
// reproduces each tell, which is what makes it a precise regression net. It is also two
// sentences long at the median, so it cannot test a signal that needs a paragraph. Cadence
// is the first such signal the engine carries, and the gate that guards every release had
// ten eligible passages to judge it with.
//
// That gap is why the rhythm work reached for the evaluation corpus, the only long text in
// the repository, and burned it. A second development corpus of full-length passages,
// collected the way the evaluation corpus is and held under the same lock, gives a
// long-range signal somewhere legitimate to be measured.

const (
	// longformMinSentences is the least a passage must hold to enter the corpus. The
	// rhythm signals need six sentences before they act at all, so a corpus built at that
	// floor would be nothing but borderline cases. Twice the floor leaves the measurement
	// free to be about the writing rather than about the minimum.
	longformMinSentences = 12
	// longformMinWords and longformMaxWords bound passage length. The band is applied to
	// both labels, since a corpus where one side is systematically longer measures the
	// collection rather than the writing.
	longformMinWords = 150
	longformMaxWords = 1200
)

// longformManifests name the files recording which repositories the evaluation corpus has
// already spent. Collection reads them so no repository lands in both corpora, which keeps
// the text lock a check on an invariant rather than the only thing maintaining it.
//
//nolint:gochecknoglobals // Immutable lookup.
var longformManifests = []string{
	"evaldata/pre2022-manifest.jsonl",
	"evaldata/pinned-manifest.jsonl",
	"evaldata/samples.jsonl",
}

// LongPassage is one line of the long-form development corpus. It carries the same fields
// as the rule exemplar corpus so both files load through one reader.
type LongPassage struct {
	// Label is "ai" or "human".
	Label string `json:"label"`
	// Note records where the passage came from, so any sample can be traced back.
	Note string `json:"note"`
	// Text is the passage itself.
	Text string `json:"text"`
}

// excludedRepos reads every repository named across the given files. It accepts the three
// shapes a spent repository is recorded in: a manifest line carrying a repo field, a
// locked sample carrying its origin in meta, and a plain line holding the name on its own.
// Reading only the manifests is what let twenty-eight locked samples back into a first
// collection run, since the human half of the evaluation corpus is recorded nowhere else.
func excludedRepos(paths ...string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			s := strings.TrimSpace(line)
			if s == "" {
				continue
			}
			if !strings.HasPrefix(s, "{") {
				out[s] = true
				continue
			}
			var row struct {
				Repo string            `json:"repo"`
				Meta map[string]string `json:"meta"`
			}
			if json.Unmarshal([]byte(s), &row) != nil {
				continue
			}
			if row.Repo != "" {
				out[row.Repo] = true
			}
			// A locked sample records where it came from as "owner/name@sha", which is the
			// only record that the evaluation corpus has spent that repository.
			if origin := row.Meta["origin"]; origin != "" {
				out[strings.SplitN(origin, "@", 2)[0]] = true
			}
		}
	}
	return out, nil
}

// longformEligible reports whether prose belongs in the corpus: long enough to carry a
// rhythm, inside the shared word band, and English enough that the rules are reading
// writing rather than a tokenizer.
func longformEligible(s *sanitize.Sanitizer, prose string) bool {
	score := s.Score(prose)
	if score.Sentences < longformMinSentences {
		return false
	}
	if score.Words < longformMinWords || score.Words > longformMaxWords {
		return false
	}
	return asciiShare(prose) >= pre2022MinASCIILetters
}

// openLongform opens the corpus for appending and returns the texts already in it, so a
// second run adds to the file rather than replacing it and never stores a passage twice.
func openLongform(path string) (f *os.File, seen map[string]bool, err error) {
	existing, err := readLines[LongPassage](path)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	seen = map[string]bool{}
	for _, p := range existing {
		seen[normalize(p.Text)] = true
	}
	f, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", path, err)
	}
	return f, seen, nil
}

// collectLongform appends human long-form passages to the corpus, read from repositories
// untouched since 2021 and never used by the evaluation corpus.
func collectLongform(want int, out string, manifests []string, w io.Writer) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	return c.collectLongformInto(want, out, manifests, w)
}

// collectLongformInto is collectLongform with the client supplied, so a test can drive the
// whole path against a local server instead of GitHub.
func (c *client) collectLongformInto(want int, out string, manifests []string, w io.Writer) error {
	excluded, err := excludedRepos(manifests...)
	if err != nil {
		return err
	}
	s, err := sanitize.New(sanitize.DefaultProfile())
	if err != nil {
		return err
	}
	f, seen, err := openLongform(out)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)

	written := 0
	for _, slice := range starSlices {
		for _, lang := range collectLanguages {
			if written >= want {
				break
			}
			hits, searchErr := c.searchRepos(lang, slice, 40)
			if searchErr != nil {
				_, _ = fmt.Fprintf(w, "search %s %s: %v\n", lang, slice, searchErr)
			}
			for _, h := range hits {
				if written >= want || excluded[h.FullName] || h.PushedAt >= collectCutoff {
					continue
				}
				text, sha, ok, readErr := c.readme(h.FullName)
				if readErr != nil || !ok {
					continue
				}
				prose := proseOnly(text)
				if !longformEligible(s, prose) || seen[normalize(prose)] {
					continue
				}
				seen[normalize(prose)] = true
				note := fmt.Sprintf("longform human: %s@%s", h.FullName, sha[:min(10, len(sha))])
				if err := enc.Encode(LongPassage{Label: "human", Note: note, Text: prose}); err != nil {
					return fmt.Errorf("write %s: %w", out, err)
				}
				written++
				_, _ = fmt.Fprintf(w, "%3d/%d %s\n", written, want, h.FullName)
			}
		}
	}
	_, _ = fmt.Fprintf(w, "appended %d human passage(s) to %s\n", written, out)
	return nil
}

// longformGenres ask for writing long enough to carry a rhythm. The short tasks the
// evaluation generator uses produce a paragraph, and a paragraph cannot be measured for
// cadence, which is the whole reason this corpus exists.
//
//nolint:gochecknoglobals // Immutable lookup.
var longformGenres = []genGenre{
	// The human half of this corpus is README prose, so the machine half has to hold
	// READMEs too. A corpus whose halves differ by genre can be separated on genre alone,
	// which reads as a strong result and measures nothing. The evaluation corpus learned
	// this the hard way and the note on matching the register in evaldata/README.md is
	// the record of it. The other genres stay because a signal that only holds on one
	// genre is worth knowing about, but the matched pair is where a number comes from.
	{"readme", "Write the README for a command line tool that watches a directory and " +
		"uploads any new file to object storage. Cover what it does, how to install it, " +
		"how to configure it, and what happens when an upload fails."},
	{"guide", "Write a getting started guide for a command line tool that formats configuration files. " +
		"Cover installation, first run, and two common mistakes."},
	{"postmortem", "Write an incident postmortem for an outage caused by an expired certificate. " +
		"Cover the timeline, the cause, and what changes afterward."},
	{"design", "Write a design document section explaining why a service should own its own database " +
		"rather than share one. Cover the trade-offs on both sides."},
	{"essay", "Write an essay about what engineers lose when code review becomes a formality."},
	{"docs", "Write the overview page of the documentation for a library that retries failed network calls."},
	{"release", "Write release notes for a version of a backup tool that changed its storage format."},
}

// generateLongform appends machine long-form passages to the corpus, walking every model,
// genre, and prompt style so no single combination decides what machine prose looks like.
// An empty genres list walks them all, and naming some restricts the run to those, which
// is how a genre added later is filled in without regenerating the rest.
func generateLongform(base string, models, genres []string, perCombo int, out string, w io.Writer) error {
	// A long-form answer takes longer than the short generator's, but it still needs a
	// ceiling. http.DefaultClient has no timeout at all, and a model that stalls mid
	// answer hangs the whole run with no output to say why.
	client := &http.Client{Timeout: 10 * time.Minute}
	return generateLongformWith(client, base, models, genres, perCombo, out, w)
}

// generateLongformWith is generateLongform with the HTTP client supplied, so a test can
// drive it against a local server instead of a model runner.
func generateLongformWith(
	client *http.Client, base string, models, genres []string, perCombo int, out string, w io.Writer,
) error {
	only := map[string]bool{}
	for _, g := range genres {
		if g = strings.TrimSpace(g); g != "" {
			only[g] = true
		}
	}
	s, err := sanitize.New(sanitize.DefaultProfile())
	if err != nil {
		return err
	}
	f, seen, err := openLongform(out)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)

	written, short := 0, 0
	for _, model := range models {
		for _, genre := range longformGenres {
			if len(only) > 0 && !only[genre.name] {
				continue
			}
			for _, style := range genStyles {
				for i := 0; i < perCombo; i++ {
					prompt := genre.task + style.instruction +
						" Write at least three hundred words. Reply with the writing only, no preamble and no title."
					text, genErr := ollamaGenerate(client, base, model, prompt)
					if genErr != nil {
						_, _ = fmt.Fprintf(w, "%s %s/%s: %v\n", model, genre.name, style.name, genErr)
						continue
					}
					prose := proseOnly(text)
					if !longformEligible(s, prose) || seen[normalize(prose)] {
						// Say so rather than moving on quietly. A model that answers
						// short every time produces a run with no output at all, which
						// reads as a hang when it is working.
						short++
						_, _ = fmt.Fprintf(w, "    set aside %s/%s/%s, %d words\n",
							model, genre.name, style.name, len(strings.Fields(prose)))
						continue
					}
					seen[normalize(prose)] = true
					note := fmt.Sprintf("longform ai: %s/%s/%s", model, genre.name, style.name)
					if err := enc.Encode(LongPassage{Label: "ai", Note: note, Text: prose}); err != nil {
						return fmt.Errorf("write %s: %w", out, err)
					}
					written++
					_, _ = fmt.Fprintf(w, "%3d %s\n", written, note)
				}
			}
		}
	}
	_, _ = fmt.Fprintf(w, "appended %d machine passage(s) to %s, %d set aside as unusable\n",
		written, out, short)
	return nil
}
