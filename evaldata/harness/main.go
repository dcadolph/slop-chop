// Command harness runs the locked evaluation corpus in evaldata/. With -check it
// validates the corpus and enforces the lock against the development corpus, which is
// what CI runs. Without flags it scores every rated sample and reports whether the slop
// score tracks blind human judgment. See evaldata/README.md for the protocol.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/dcadolph/slop-chop/sanitize"
)

// devPassage is one line of the development corpus, read only for the disjointness lock.
type devPassage struct {
	// Text is the passage text.
	Text string `json:"text"`
}

func main() {
	check := flag.Bool("check", false, "validate the corpus and the lock, then exit")
	flag.Parse()

	samples, err := readLines[Sample]("evaldata/samples.jsonl")
	if err != nil {
		fatal(err)
	}
	ratings, err := readLines[Rating]("evaldata/ratings.jsonl")
	if err != nil {
		fatal(err)
	}
	dev, err := readLines[devPassage]("sanitize/testdata/corpus.jsonl")
	if err != nil {
		fatal(err)
	}
	devTexts := make([]string, 0, len(dev))
	for _, p := range dev {
		devTexts = append(devTexts, p.Text)
	}

	problems := append(checkCorpus(samples, devTexts), checkRatings(ratings, samples)...)
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "evaldata:", p)
		}
		os.Exit(1)
	}
	if *check {
		fmt.Printf("evaldata: %d sample(s) and %d rating(s), lock holds\n", len(samples), len(ratings))
		return
	}

	if len(samples) == 0 {
		fmt.Println("evaldata: no samples yet; see evaldata/README.md for the collection protocol")
		return
	}
	s, err := sanitize.New(sanitize.DefaultProfile())
	if err != nil {
		fatal(err)
	}
	rows := scoreSamples(s, samples, ratings)
	if len(rows) == 0 {
		fmt.Println("evaldata: samples exist but none are rated yet; see the rating protocol")
		return
	}
	var b strings.Builder
	report(&b, rows, ratings)
	fmt.Print(b.String())
}

// fatal prints the error and exits non-zero.
func fatal(err error) {
	fmt.Fprintln(os.Stderr, "evaldata:", err)
	os.Exit(1)
}
