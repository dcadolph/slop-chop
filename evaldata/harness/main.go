// Command harness runs the locked evaluation corpus in evaldata/. With -check it
// validates the corpus and enforces the lock against the development corpus, which is
// what CI runs. Without flags it scores every rated sample and reports whether the slop
// score tracks blind human judgment. See evaldata/README.md for the protocol.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dcadolph/slop-chop/sanitize"
)

// devPassage is one line of the development corpus, read only for the disjointness lock.
type devPassage struct {
	// Text is the passage text.
	Text string `json:"text"`
}

// paths names the three corpora the harness reads, so a test can point the run at its
// own files instead of the repository's.
type paths struct {
	// samples is the locked evaluation corpus.
	samples string
	// ratings is the blind human ratings for those samples.
	ratings string
	// dev is the development corpus the lock keeps the samples away from.
	dev string
}

// defaultPaths returns the repository's corpora, which is what a plain run reads.
func defaultPaths() paths {
	return paths{
		samples: "evaldata/samples.jsonl",
		ratings: "evaldata/ratings.jsonl",
		dev:     "sanitize/testdata/corpus.jsonl",
	}
}

// errCorpus means the corpus or its ratings broke a rule. Its text is the problem list,
// already formatted one per line.
var errCorpus = errors.New("corpus problems")

func main() {
	check := flag.Bool("check", false, "validate the corpus and the lock, then exit")
	gather := flag.Int("collect-pre2022", 0, "collect N READMEs from repositories untouched since 2021")
	pinned := flag.Int("collect-pinned", 0, "collect N READMEs from maintained repositories, read at their last pre-2022 commit")
	falsePos := flag.Bool("pre2022", false, "score the pre-2022 READMEs and report the false-positive rate")
	manifest := flag.String("pre2022-manifest", "", "also write a text-free manifest of the scored samples here")
	corpus := flag.String("pre2022-file", "evaldata/pre2022.jsonl", "where the pre-2022 READMEs are read from and written to")
	flag.Parse()

	var err error
	switch {
	case *gather > 0:
		err = collect(*gather, *corpus, os.Stdout)
	case *pinned > 0:
		err = collectPinned(*pinned, *corpus, os.Stdout)
	case *falsePos:
		err = runPre2022(*corpus, *manifest, os.Stdout)
	default:
		err = run(*check, defaultPaths(), os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runPre2022 scores the collected READMEs and writes the false-positive measurement to w.
func runPre2022(path, manifestPath string, w io.Writer) error {
	readmes, err := readLines[Readme](path)
	if err != nil {
		return err
	}
	s, err := sanitize.New(sanitize.DefaultProfile())
	if err != nil {
		return fmt.Errorf("sanitizer: %w", err)
	}
	results, skipped := scoreReadmes(s, readmes)
	if manifestPath != "" {
		if err := writeManifest(manifestPath, manifestOf(readmes, results)); err != nil {
			return err
		}
	}
	var b strings.Builder
	pre2022Report(&b, results, skipped, provenanceOf(readmes), languageSpread(results))
	_, err = io.WriteString(w, b.String())
	return err
}

// run validates the corpora and, unless check is set, writes the analysis to w. Every
// problem is reported at once rather than one per run, so a corpus is fixed in one pass.
func run(check bool, p paths, w io.Writer) error {
	samples, err := readLines[Sample](p.samples)
	if err != nil {
		return err
	}
	ratings, err := readLines[Rating](p.ratings)
	if err != nil {
		return err
	}
	dev, err := readLines[devPassage](p.dev)
	if err != nil {
		return err
	}
	devTexts := make([]string, 0, len(dev))
	for _, d := range dev {
		devTexts = append(devTexts, d.Text)
	}

	if problems := append(checkCorpus(samples, devTexts), checkRatings(ratings, samples)...); len(problems) > 0 {
		var b strings.Builder
		for _, problem := range problems {
			fmt.Fprintf(&b, "evaldata: %s\n", problem)
		}
		return fmt.Errorf("%w:\n%s", errCorpus, strings.TrimRight(b.String(), "\n"))
	}
	if check {
		_, err := fmt.Fprintf(w, "evaldata: %d sample(s) and %d rating(s), lock holds\n",
			len(samples), len(ratings))
		return err
	}

	if len(samples) == 0 {
		_, err := fmt.Fprintln(w,
			"evaldata: no samples yet; see evaldata/README.md for the collection protocol")
		return err
	}
	s, err := sanitize.New(sanitize.DefaultProfile())
	if err != nil {
		return err
	}
	rows := scoreSamples(s, samples, ratings)
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w,
			"evaldata: samples exist but none are rated yet; see the rating protocol")
		return err
	}
	var b strings.Builder
	report(&b, rows, ratings)
	_, err = io.WriteString(w, b.String())
	return err
}

// writeManifest writes one manifest row per line, so a published result can be audited
// against the commits it was measured on.
func writeManifest(path string, rows []Manifest) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}
