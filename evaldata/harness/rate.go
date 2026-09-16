package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Blind rating is the step the locked corpus has been waiting on. The protocol asks for
// people to read samples without knowing which are machine-written, and nothing here
// presented them that way, so the corpus sat collected and unrated. This does the
// presenting: samples in a fixed shuffle, labels withheld, one answer each, appended to
// the ratings file. It never shows the source, never scores anything, and never reveals
// what the engine thought, because a rater who can see any of that is no longer blind.

// ratingPrompt is what a rater answers. The wording asks for a reading rather than a
// verdict on authorship, since a rater cannot know who wrote it either.
const ratingPrompt = "How machine-written does this read?  1 clearly a person ... 7 clearly a machine"

// rateShuffle reorders samples deterministically from a seed the rater supplies, so two
// raters see different orders while either one can be reproduced. It is a small
// multiplicative walk rather than a real shuffle, which is enough to break the collection
// order that groups every machine sample together.
func rateShuffle(samples []Sample, seed int) []Sample {
	out := make([]Sample, len(samples))
	copy(out, samples)
	if len(out) < 2 {
		return out
	}
	// A step coprime with the length visits every index exactly once.
	step := seed%len(out) + 1
	for gcd(step, len(out)) != 1 {
		step++
	}
	shuffled := make([]Sample, 0, len(out))
	for i, at := 0, seed%len(out); i < len(out); i++ {
		shuffled = append(shuffled, out[at])
		at = (at + step) % len(out)
	}
	return shuffled
}

// gcd returns the greatest common divisor of a and b.
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// rated returns the set of sample ids this rater has already answered, so a session can
// be stopped and resumed without asking the same question twice.
func rated(ratings []Rating, rater string) map[string]bool {
	out := map[string]bool{}
	for _, r := range ratings {
		if r.Rater == rater {
			out[r.Sample] = true
		}
	}
	return out
}

// runRate walks the unrated samples with the given rater and appends each answer. Reading
// stops on end of input or on the quit command, and every answer is written as it is
// given, so a session that ends early keeps what it collected.
func runRate(samplesPath, ratingsPath, rater string, seed int, in io.Reader, out io.Writer) error {
	if strings.TrimSpace(rater) == "" {
		return fmt.Errorf("a rater id is required, so a rating can be attributed")
	}
	samples, err := readLines[Sample](samplesPath)
	if err != nil {
		return err
	}
	if len(samples) == 0 {
		return fmt.Errorf("%s holds no samples", samplesPath)
	}
	// A missing ratings file is the ordinary first run, not a failure.
	ratings, err := readLines[Rating](ratingsPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	done := rated(ratings, rater)

	f, err := os.OpenFile(ratingsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", ratingsPath, err)
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)

	queue := rateShuffle(samples, seed)
	remaining := 0
	for _, s := range queue {
		if !done[s.ID] {
			remaining++
		}
	}
	_, _ = fmt.Fprintf(out, "%d samples, %d left for rater %q.\n", len(samples), remaining, rater)
	_, _ = fmt.Fprintf(out, "Answer 1 to 7, or q to stop. Nothing here tells you the answer.\n\n")

	reader := bufio.NewReader(in)
	answered := 0
	for _, s := range queue {
		if done[s.ID] {
			continue
		}
		_, _ = fmt.Fprintf(out, "%s\n\n%s\n\n%s\n> ", strings.Repeat("-", 72), strings.TrimSpace(s.Text), ratingPrompt)
		line, readErr := reader.ReadString('\n')
		answer := strings.TrimSpace(line)
		if answer == "q" || answer == "quit" || (readErr != nil && answer == "") {
			break
		}
		n, convErr := strconv.Atoi(answer)
		if convErr != nil || n < 1 || n > 7 {
			_, _ = fmt.Fprintf(out, "  not a 1 to 7 answer, skipping\n\n")
			if readErr != nil {
				break
			}
			continue
		}
		if err := enc.Encode(Rating{Sample: s.ID, Rater: rater, Machine: n}); err != nil {
			return fmt.Errorf("write %s: %w", ratingsPath, err)
		}
		answered++
		if readErr != nil {
			break
		}
	}
	_, _ = fmt.Fprintf(out, "\n%d rating(s) recorded in %s\n", answered, ratingsPath)
	return nil
}
