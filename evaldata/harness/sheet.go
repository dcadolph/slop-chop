package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Rating by hand needs the repository, a Go toolchain, and a terminal, which is three
// barriers between the corpus and the people whose judgment it is collecting. A rater is
// someone doing a favor, not someone setting up a development environment. These two
// commands take the corpus out to a spreadsheet and bring the answers back, so the only
// thing asked of a rater is to read and type a number.

// sheetHeader is the column order of the exported sheet. The id comes first so a returned
// file can be matched back even if the rows are sorted, and the answer column is last
// because that is where a person's eye lands after reading.
//
//nolint:gochecknoglobals // Immutable lookup.
var sheetHeader = []string{"key", "text", "machine_1_to_7"}

// sheetKey is the opaque handle a rater sees instead of the sample id. The ids encode
// provenance in their first character, h for human and a for machine, so a sheet carrying
// them hands over the answer to anyone who notices the pattern in the first two rows. The
// key is derived from the id, so a returned sheet still matches without a mapping file.
func sheetKey(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])[:10]
}

// exportSheet writes the samples to CSV in a shuffled order, one row each, with an empty
// answer column. The source label is never written: a sheet that carries it is not a blind
// instrument, and a spreadsheet is exactly the kind of file somebody scrolls sideways in.
func exportSheet(samplesPath, out string, seed int) error {
	samples, err := readLines[Sample](samplesPath)
	if err != nil {
		return err
	}
	if len(samples) == 0 {
		return fmt.Errorf("%s holds no samples", samplesPath)
	}
	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("create %s: %w", out, err)
	}
	defer func() { _ = f.Close() }()

	w := csv.NewWriter(f)
	if err := w.Write(sheetHeader); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	for _, s := range rateShuffle(samples, seed) {
		if err := w.Write([]string{sheetKey(s.ID), strings.TrimSpace(s.Text), ""}); err != nil {
			return fmt.Errorf("write %s: %w", out, err)
		}
	}
	w.Flush()
	return w.Error()
}

// importSheet reads a filled sheet back and appends one rating per answered row. A blank
// answer is a row the rater skipped and is passed over rather than guessed at. An answer
// outside one to seven is reported and skipped, since a corpus quietly holding a rating
// nobody gave is worse than a corpus missing one.
func importSheet(sheet, samplesPath, ratingsPath, rater string, w io.Writer) error {
	if strings.TrimSpace(rater) == "" {
		return fmt.Errorf("a rater id is required, so a rating can be attributed")
	}
	samples, err := readLines[Sample](samplesPath)
	if err != nil {
		return err
	}
	bySheetKey := make(map[string]string, len(samples))
	for _, s := range samples {
		bySheetKey[sheetKey(s.ID)] = s.ID
	}
	f, openErr := os.Open(sheet)
	if openErr != nil {
		return fmt.Errorf("open %s: %w", sheet, openErr)
	}
	defer func() { _ = f.Close() }()

	rows, readErr := csv.NewReader(f).ReadAll()
	if readErr != nil {
		return fmt.Errorf("read %s: %w", sheet, readErr)
	}
	if len(rows) < 2 {
		return fmt.Errorf("%s holds no rows", sheet)
	}
	id, answer, colErr := sheetColumns(rows[0])
	if colErr != nil {
		return colErr
	}

	out, outErr := os.OpenFile(ratingsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if outErr != nil {
		return fmt.Errorf("open %s: %w", ratingsPath, outErr)
	}
	defer func() { _ = out.Close() }()
	enc := json.NewEncoder(out)

	added, skipped := 0, 0
	for i, row := range rows[1:] {
		if id >= len(row) || answer >= len(row) {
			continue
		}
		raw := strings.TrimSpace(row[answer])
		if raw == "" {
			continue
		}
		n, convErr := strconv.Atoi(raw)
		if convErr != nil || n < 1 || n > 7 {
			_, _ = fmt.Fprintf(w, "row %d (%s): %q is not a 1 to 7 answer, skipped\n", i+2, row[id], raw)
			skipped++
			continue
		}
		sample, known := bySheetKey[strings.TrimSpace(row[id])]
		if !known {
			_, _ = fmt.Fprintf(w, "row %d: %q matches no sample in the corpus, skipped\n", i+2, row[id])
			skipped++
			continue
		}
		if err := enc.Encode(Rating{Sample: sample, Rater: rater, Machine: n}); err != nil {
			return fmt.Errorf("write %s: %w", ratingsPath, err)
		}
		added++
	}
	_, _ = fmt.Fprintf(w, "added %d rating(s) from %s as %q, skipped %d\n", added, sheet, rater, skipped)
	return nil
}

// sheetColumns locates the id and answer columns by header name, so a rater who reorders
// or adds columns in a spreadsheet does not silently shift the answers onto the wrong
// samples.
func sheetColumns(header []string) (id, answer int, err error) {
	id, answer = -1, -1
	for i, h := range header {
		switch strings.TrimSpace(strings.ToLower(h)) {
		case "key", "id":
			id = i
		case "machine_1_to_7":
			answer = i
		}
	}
	if id < 0 || answer < 0 {
		return 0, 0, fmt.Errorf("sheet needs a %q and a %q column, got %v", "key", "machine_1_to_7", header)
	}
	return id, answer, nil
}
