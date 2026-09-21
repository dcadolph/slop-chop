package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
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

// anthropicBase is the Messages API root. It is a variable so a test can point at a local
// server, and it is the only thing about this path that is not plain HTTP.
//
//nolint:gochecknoglobals // Overridden only by tests.
var anthropicBase = "https://api.anthropic.com"

// anthropicVersionHeader pins the wire format so the request shape cannot drift under the
// corpus, which would change what the samples are without anything recording it.
const anthropicVersionHeader = "2023-06-01"

// isAnthropicModel reports whether a model name should be generated through the Messages
// API rather than through the local daemon. Dispatching on the name keeps one flag doing
// the work: -generate-models claude-opus-4-8,llama3.2:3b mixes both in one run.
func isAnthropicModel(model string) bool {
	return strings.HasPrefix(model, "claude-")
}

// anthropicGenerate asks a frontier model for one completion. The corpus has only ever
// held prose from small local models, which span nothing to thirty-eight percent against
// the same rules, so a number measured without a frontier model in the sample says little
// about the writing people mean. This is the path that closes that gap.
func anthropicGenerate(client *http.Client, key, model, prompt string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY is not set")
	}
	body, err := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 2048,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
	})
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, anthropicBase+"/v1/messages", strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", anthropicVersionHeader)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("anthropic read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic: %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var out struct {
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("anthropic decode: %w", err)
	}
	// A truncated reply is a sample the model did not finish writing, and a refusal is no
	// sample at all. Either one silently entering the corpus would be a passage nobody
	// actually produced, so both are errors and the caller discards them.
	if out.StopReason != "end_turn" {
		return "", fmt.Errorf("anthropic: stop_reason %q", out.StopReason)
	}
	var b strings.Builder
	for _, block := range out.Content {
		if block.Type == "text" {
			b.WriteString(block.Text)
		}
	}
	return strings.TrimSpace(b.String()), nil
}

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
					var text string
					var genErr error
					if isAnthropicModel(model) {
						text, genErr = anthropicGenerate(client, os.Getenv("ANTHROPIC_API_KEY"), model, prompt)
					} else {
						text, genErr = ollamaGenerate(client, base, model, prompt)
					}
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

// Human samples have to match the machine samples in register or the rating experiment
// measures the wrong thing. A corpus of nineteenth century essays against modern work
// email lets a rater separate the halves on century and genre alone, and score a perfect
// result without ever judging whether prose reads machine-written. These are drawn from
// README files written before 2022, which is the one modern human register available in
// volume with a hard date guarantee.

// humanExcludeFile names repositories already spent on the false-positive measurement.
// Those were used to evaluate a candidate rule change, so the lock keeps them out of the
// corpus that has to stay untouched.
func loadExcluded(path string) (map[string]bool, error) {
	out := map[string]bool{}
	if path == "" {
		return out, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			out[s] = true
		}
	}
	return out, nil
}

// collectHumanREADMEs appends register-matched human samples to the locked corpus, taken
// from repositories untouched since 2021 and never used for anything else. The prose band
// and the language filter match the machine half, so neither side is selected for length.
func collectHumanREADMEs(want int, samplesPath, excludePath, rulesTag string, w io.Writer) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	return c.collectHumanInto(want, samplesPath, excludePath, rulesTag, w)
}

// collectHumanInto is collectHumanREADMEs with the client supplied, so a test can drive
// the whole path against a local server instead of GitHub.
func (c *client) collectHumanInto(want int, samplesPath, excludePath, rulesTag string, w io.Writer) error {
	excluded, err := loadExcluded(excludePath)
	if err != nil {
		return err
	}
	existing, err := readLines[Sample](samplesPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	next := 1
	for _, s := range existing {
		if strings.HasPrefix(s.ID, "h") {
			next++
		}
	}

	f, err := os.OpenFile(samplesPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", samplesPath, err)
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
				// A README is mostly badges and fences. Take the prose paragraphs only,
				// so the sample is writing rather than markup.
				prose := proseOnly(text)
				if n := len(strings.Fields(prose)); n < genMinWords || n > genMaxWords {
					continue
				}
				if asciiShare(prose) < pre2022MinASCIILetters {
					continue
				}
				if err := enc.Encode(Sample{
					ID:     fmt.Sprintf("h%03d", next),
					Source: "human",
					Rules:  rulesTag,
					Meta: map[string]string{
						"origin": h.FullName + "@" + sha[:min(10, len(sha))],
						"genre":  "readme",
					},
					Text: prose,
				}); err != nil {
					return fmt.Errorf("write %s: %w", samplesPath, err)
				}
				excluded[h.FullName] = true
				next++
				written++
				_, _ = fmt.Fprintf(w, "h%03d <- %s\n", next-1, h.FullName)
			}
		}
	}
	_, _ = fmt.Fprintf(w, "wrote %d human sample(s)\n", written)
	return nil
}

// proseOnly strips a README down to its prose paragraphs. Fences, headings, badges,
// tables, and lists go, and so does the inline markup inside the lines that survive: a
// sample still carrying link syntax or a code span is distinguishable from a generated one
// on markup alone, which would hand a rater the answer as surely as the label would.
func proseOnly(text string) string {
	var out []string
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || skipLine(trimmed) {
			continue
		}
		clean := strings.TrimSpace(stripInline(trimmed))
		// Whatever is left has to read as a sentence rather than as the remains of one.
		if len(strings.Fields(clean)) < 4 || strings.ContainsAny(clean, "|<>`") {
			continue
		}
		out = append(out, clean)
	}
	return strings.TrimSpace(strings.Join(out, " "))
}

// skipLine reports whether a README line is structure rather than prose.
func skipLine(trimmed string) bool {
	if trimmed == "" {
		return true
	}
	for _, prefix := range []string{"#", "|", ">", "-", "*", "<", "[", "!", "    ", "\t"} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	// A numbered list item is a list whatever it starts with.
	if len(trimmed) > 1 && trimmed[0] >= '0' && trimmed[0] <= '9' {
		rest := strings.TrimLeft(trimmed, "0123456789")
		if strings.HasPrefix(rest, ".") || strings.HasPrefix(rest, ")") {
			return true
		}
	}
	return false
}

// stripInline removes the markup that survives inside a prose line: link and image
// syntax, code spans, and emphasis markers. A link keeps its visible text and loses its
// destination, since the text is the writing and the URL is not.
func stripInline(line string) string {
	line = inlineImageRe.ReplaceAllString(line, "")
	line = inlineLinkRe.ReplaceAllString(line, "$1")
	line = codeSpanRe.ReplaceAllString(line, "")
	line = bareURLInline.ReplaceAllString(line, "")
	return strings.NewReplacer("**", "", "__", "", "*", "", "_", "").Replace(line)
}

//nolint:gochecknoglobals // Compiled once, never modified.
var (
	inlineImageRe = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	inlineLinkRe  = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	codeSpanRe    = regexp.MustCompile("`[^`]*`")
	bareURLInline = regexp.MustCompile(`https?://\S+`)
)
