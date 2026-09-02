// Command harness runs the locked evaluation corpus in evaldata/. With -check it
// validates the corpus and enforces the lock against the development corpus, which is
// what CI runs. Without flags it scores every rated sample and reports whether the slop
// score tracks blind human judgment. See evaldata/README.md for the protocol.
package main

import (
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
	flag.Parse()
	if err := run(*check, defaultPaths(), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
