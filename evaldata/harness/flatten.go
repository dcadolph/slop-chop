package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// flattenCorpus puts every sample that carries markup or layout through proseOnly, the
// filter the human half was collected with, and drops any it leaves outside the length
// band or still marked. Clean samples pass through untouched. It reads no scores, so it
// cannot burn the corpus, and it refuses to run once a rating exists, since a rated text
// is frozen.
func flattenCorpus(samplesPath, ratingsPath string, w io.Writer) error {
	ratings, err := readLines[Rating](ratingsPath)
	if err != nil {
		return err
	}
	if len(ratings) > 0 {
		return fmt.Errorf("%s holds %d rating(s): a rated sample is frozen and cannot be flattened",
			ratingsPath, len(ratings))
	}
	samples, err := readLines[Sample](samplesPath)
	if err != nil {
		return err
	}
	kept := make([]Sample, 0, len(samples))
	flattened, dropped := 0, 0
	for _, s := range samples {
		if markupLeak(s.Text) == "" {
			kept = append(kept, s)
			continue
		}
		s.Text = proseOnly(s.Text)
		n := len(strings.Fields(s.Text))
		if n < genMinWords || n > genMaxWords || markupLeak(s.Text) != "" {
			dropped++
			_, _ = fmt.Fprintf(w, "%s dropped: %d words of prose left\n", s.ID, n)
			continue
		}
		flattened++
		kept = append(kept, s)
	}
	if err := writeSamples(samplesPath, kept); err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "flattened %d sample(s), dropped %d, %d remain\n", flattened, dropped, len(kept))
	return err
}

// writeSamples replaces the corpus file through a rename, so an interrupted write never
// leaves a half-written corpus behind.
func writeSamples(path string, samples []Sample) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".samples-*.jsonl")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", path, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	enc := json.NewEncoder(tmp)
	for _, s := range samples {
		if err := enc.Encode(s); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp for %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
