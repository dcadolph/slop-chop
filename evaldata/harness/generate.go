package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Machine samples for the locked corpus are generated here, locally, through ollama. The
// protocol asks for several model families and a spread of prompt styles, including the
// adversarial ask that tells the model to avoid the cliches, since that is the prose the
// ruleset is most likely to miss. Nothing generated here is read while writing a rule:
// the samples go straight into the corpus and stay unexamined until they are rated.

// genMinWords and genMaxWords are the length band the protocol sets for a sample. A reply
// outside it is discarded rather than trimmed, since trimming would edit the model's prose
// and the sample is supposed to be what the model actually wrote.
const (
	genMinWords = 60
	genMaxWords = 400
)

// genGenre is one kind of writing to ask for.
type genGenre struct {
	// name is recorded in the sample's meta.
	name string
	// task is the thing the model is asked to write.
	task string
}

// genStyle is one way of asking, which is the axis that matters most: a model told to
// avoid the cliches writes the prose a word list cannot catch.
type genStyle struct {
	// name is recorded in the sample's meta.
	name string
	// instruction is appended to the task.
	instruction string
}

//nolint:gochecknoglobals // Immutable lookups.
var (
	genGenres = []genGenre{
		{"email", "Write a short work email announcing that a planned maintenance window has moved."},
		{"readme", "Write the introduction section of a README for a small command line tool that converts CSV to JSON."},
		{"blog", "Write a few paragraphs of a blog post about why small teams should write down decisions."},
		{"report", "Write a short status report on a project that slipped by two weeks because of a dependency upgrade."},
		{"chat", "Write a reply to a colleague who asked whether they should add a cache in front of their database."},
	}

	genStyles = []genStyle{
		{"plain", ""},
		{"styled", " Write it in a confident, polished, professional voice."},
		{"adversarial", " Avoid every cliche of AI writing. Use no em-dashes, no stock openers, no words like comprehensive, robust, leverage, or delve, and vary your sentence rhythm so it does not read as machine-written."},
	}
)

// ollamaGenerate asks a local model for one completion. It talks to the daemon directly
// rather than shelling out, so a failure comes back as an error instead of as text.
func ollamaGenerate(client *http.Client, base, model, prompt string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	})
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}
	resp, err := client.Post(base+"/api/generate", "application/json", strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("ollama: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ollama read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama: %s", resp.Status)
	}
	var out struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("ollama decode: %w", err)
	}
	return strings.TrimSpace(out.Response), nil
}

// generateAI writes machine samples into path, walking genres and prompt styles across
// every model so no one combination dominates the corpus. Existing samples are read first
// so ids continue rather than collide, and the file is appended rather than replaced.
func generateAI(models []string, perCombo int, path, rulesTag string, w io.Writer) error {
	return generateAIFrom(ollamaBase, models, perCombo, path, rulesTag, w)
}

// ollamaBase is where the local daemon listens.
const ollamaBase = "http://localhost:11434"

// generateAIFrom is generateAI with the daemon address supplied, so a test can drive the
// whole path against a local server instead of a model.
func generateAIFrom(base string, models []string, perCombo int, path, rulesTag string, w io.Writer) error {
	existing, err := readLines[Sample](path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	next := 1
	for _, s := range existing {
		if strings.HasPrefix(s.ID, "a") {
			next++
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	client := &http.Client{Timeout: 5 * time.Minute}

	written, skipped := 0, 0
	for _, model := range models {
		for _, genre := range genGenres {
			for _, style := range genStyles {
				for range perCombo {
					prompt := genre.task + style.instruction + " Reply with the writing only, no preamble and no title."
					text, genErr := ollamaGenerate(client, base, model, prompt)
					if genErr != nil {
						_, _ = fmt.Fprintf(w, "%s %s/%s: %v\n", model, genre.name, style.name, genErr)
						continue
					}
					if n := len(strings.Fields(text)); n < genMinWords || n > genMaxWords {
						skipped++
						continue
					}
					sample := Sample{
						ID:     fmt.Sprintf("a%03d", next),
						Source: "ai",
						Rules:  rulesTag,
						Meta: map[string]string{
							"model": model, "prompt": style.name, "genre": genre.name,
						},
						Text: text,
					}
					if err := enc.Encode(sample); err != nil {
						return fmt.Errorf("write %s: %w", path, err)
					}
					next++
					written++
					_, _ = fmt.Fprintf(w, "%s %s/%s -> %s\n", model, genre.name, style.name, sample.ID)
				}
			}
		}
	}
	_, _ = fmt.Fprintf(w, "wrote %d machine sample(s), discarded %d outside %d to %d words\n",
		written, skipped, genMinWords, genMaxWords)
	return nil
}
