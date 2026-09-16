package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Collection pulls the pre-2022 sample from GitHub. The search is sliced by star range
// because the search API caps any one query at a thousand results, and the slices are
// disjoint so no repository is collected twice.

// collectCutoff is the push date a repository must predate. A repository untouched since
// before this carries a README written before general writing models, which is the whole
// basis of the human guarantee.
const collectCutoff = "2022-01-01"

// starSlices partition the search so each query stays under the thousand-result cap the
// API enforces. The ranges are disjoint and ascending.
//
//nolint:gochecknoglobals // Immutable lookup.
var starSlices = []string{
	"50..74", "75..99", "100..149", "150..249", "250..399",
	"400..699", "700..1199", "1200..2499", "2500..*",
}

// collectLanguages spread the sample across ecosystems. One language would measure the
// README conventions of a single community, and a false-positive rate that only holds for
// Go projects is not evidence about professional prose.
//
//nolint:gochecknoglobals // Immutable lookup.
var collectLanguages = []string{"Go", "Python", "Rust", "JavaScript", "Java", "Ruby", "C++"}

// errNotFound means the API answered 404. Callers separate it from a real failure: a
// repository with no README is a sample to skip, while a rate limit or a network error is
// a hole in the measurement and must not be mistaken for an absent file.
var errNotFound = errors.New("not found")

// githubToken returns a token for the API, preferring the environment and falling back to
// the gh CLI's stored credential so a developer already logged in needs no setup.
func githubToken() (string, error) {
	for _, key := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if v := os.Getenv(key); v != "" {
			return v, nil
		}
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("no GITHUB_TOKEN and gh auth token failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// repoHit is the part of a search result the collection keeps.
type repoHit struct {
	// FullName is the owner and name.
	FullName string `json:"full_name"`
	// PushedAt is the last push, checked against the cutoff.
	PushedAt string `json:"pushed_at"`
	// Stars is the star count.
	Stars int `json:"stargazers_count"`
	// DefaultBranch is the branch the README is read from.
	DefaultBranch string `json:"default_branch"`
}

// client carries the token and a shared HTTP client across the collection calls.
type client struct {
	// token authenticates every request.
	token string
	// http is the underlying client, with a timeout so one slow repository cannot hang
	// a run of a thousand.
	http *http.Client
	// base is the API root. It is a field so a test can point the client at a local
	// server instead of GitHub.
	base string
	// pause is how long to wait between search pages. The search endpoint is limited far
	// more tightly than the rest of the API, and a test sets this to zero.
	pause time.Duration
}

// newClient builds a collection client from the ambient GitHub credentials.
func newClient() (*client, error) {
	token, err := githubToken()
	if err != nil {
		return nil, err
	}
	return &client{
		token: token,
		http:  &http.Client{Timeout: 30 * time.Second},
		base:  "https://api.github.com",
		pause: 2 * time.Second,
	}, nil
}

// get issues one authenticated GET and decodes the JSON body into v. A rate-limit reply
// is retried once after the window the response names, since a thousand-sample run will
// meet the secondary limit at least once.
func (c *client) get(rawURL string, v any) error {
	for attempt := range 2 {
		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			return fmt.Errorf("request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("get %s: %w", rawURL, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read %s: %w", rawURL, readErr)
		}
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			if attempt == 0 {
				time.Sleep(30 * c.pause)
				continue
			}
		}
		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("get %s: %w", rawURL, errNotFound)
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("get %s: %s", rawURL, resp.Status)
		}
		if err := json.Unmarshal(body, v); err != nil {
			return fmt.Errorf("decode %s: %w", rawURL, err)
		}
		return nil
	}
	return fmt.Errorf("get %s: rate limited twice", rawURL)
}

// searchRepos returns repositories in one language and star slice that have not been
// pushed since the cutoff, walking pages until the slice is exhausted or want is reached.
func (c *client) searchRepos(lang, slice string, want int) ([]repoHit, error) {
	var out []repoHit
	for page := 1; page <= 10 && len(out) < want; page++ {
		q := fmt.Sprintf("language:%s pushed:<%s stars:%s", lang, collectCutoff, slice)
		u := fmt.Sprintf("%s/search/repositories?q=%s&per_page=100&page=%d&sort=stars",
			c.base, url.QueryEscape(q), page)
		var page struct {
			Items []repoHit `json:"items"`
		}
		if err := c.get(u, &page); err != nil {
			return out, err
		}
		if len(page.Items) == 0 {
			break
		}
		out = append(out, page.Items...)
		time.Sleep(c.pause)
	}
	return out, nil
}

// readme fetches a repository's README along with the commit it was read at. A repository
// with no README is not an error; it comes back with ok false and is simply skipped.
func (c *client) readme(repo string) (text, sha string, ok bool, err error) {
	var payload struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
		SHA      string `json:"sha"`
	}
	if err := c.get(c.base+"/repos/"+repo+"/readme", &payload); err != nil {
		if errors.Is(err, errNotFound) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	if payload.Encoding != "base64" {
		return "", "", false, nil
	}
	raw, decErr := base64.StdEncoding.DecodeString(strings.ReplaceAll(payload.Content, "\n", ""))
	if decErr != nil {
		return "", "", false, fmt.Errorf("decode readme %s: %w", repo, decErr)
	}
	return string(raw), payload.SHA, true, nil
}

// collect gathers up to want READMEs from repositories untouched since the cutoff and
// writes them to path as JSONL. Progress goes to w one repository at a time, since a
// run of a thousand takes minutes and a silent wait reads as a hang.
func collect(want int, path string, w io.Writer) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	return c.collectInto(want, path, w)
}

// collectInto is collect with the client supplied, so a test can drive the whole path
// against a local server instead of GitHub.
func (c *client) collectInto(want int, path string, w io.Writer) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	enc := json.NewEncoder(f)
	seen := map[string]bool{}
	written := 0
	// Each language and star slice gets the same budget, with headroom for the hits that
	// carry no README or fail the language filter. Without the cap the first language
	// would spend the whole sample and the spread across ecosystems would be a fiction.
	perSlice := want/(len(starSlices)*len(collectLanguages)) + 3

	// Star tier is the outer loop and language the inner one, so every ecosystem gets a
	// turn at each tier before any ecosystem gets a second helping. The other order lets
	// the first language take its whole budget while the last one never runs.
	for _, slice := range starSlices {
		for _, lang := range collectLanguages {
			if written >= want {
				break
			}
			hits, searchErr := c.searchRepos(lang, slice, perSlice)
			if searchErr != nil {
				_, _ = fmt.Fprintf(w, "search %s %s: %v\n", lang, slice, searchErr)
			}
			if len(hits) > perSlice {
				hits = hits[:perSlice]
			}
			written = c.writeHits(lang, hits, seen, enc, w, want, written)
		}
	}
	_, _ = fmt.Fprintf(w, "collected %d READMEs into %s\n", written, path)
	return nil
}

// writeHits fetches and writes the README for each unseen hit, returning the new total.
func (c *client) writeHits(lang string, hits []repoHit, seen map[string]bool, enc *json.Encoder, w io.Writer, want, written int) int {
	for _, h := range hits {
		if written >= want || seen[h.FullName] {
			continue
		}
		seen[h.FullName] = true
		// The search filter already excludes later pushes, but the field is the
		// guarantee the whole measurement rests on, so it is checked again here.
		if h.PushedAt >= collectCutoff {
			continue
		}
		text, sha, ok, readErr := c.readme(h.FullName)
		if readErr != nil {
			_, _ = fmt.Fprintf(w, "readme %s: %v\n", h.FullName, readErr)
			continue
		}
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		if err := enc.Encode(Readme{
			Repo:     h.FullName,
			SHA:      sha,
			PushedAt: h.PushedAt[:10],
			Stars:    h.Stars,
			Language: lang,
			Text:     text,
		}); err != nil {
			_, _ = fmt.Fprintf(w, "write %s: %v\n", h.FullName, err)
			continue
		}
		written++
		if written%25 == 0 {
			_, _ = fmt.Fprintf(w, "collected %d/%d\n", written, want)
		}
	}
	return written
}

// Pinned collection removes the selection bias in the push-date sample. A repository with
// no push since 2021 is an abandoned one, and abandoned projects may write READMEs
// differently from maintained projects, in a direction that could flatter the engine. The
// pinned sample fixes that: it takes active, popular repositories and reads their README
// as it stood at the last commit before the cutoff. The prose is still guaranteed to
// predate general writing models, but the projects are no longer selected for being dead.

// lastCommitBefore returns the newest commit on the default branch dated before the
// cutoff. It reports ok false when the repository had no commits that early.
func (c *client) lastCommitBefore(repo, cutoff string) (sha string, ok bool, err error) {
	var commits []struct {
		SHA string `json:"sha"`
	}
	u := fmt.Sprintf("%s/repos/%s/commits?until=%sT00:00:00Z&per_page=1", c.base, repo, cutoff)
	if err := c.get(u, &commits); err != nil {
		return "", false, err
	}
	if len(commits) == 0 {
		return "", false, nil
	}
	return commits[0].SHA, true, nil
}

// readmeAt fetches a repository's README as it stood at one commit.
func (c *client) readmeAt(repo, ref string) (text string, ok bool, err error) {
	var payload struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	u := fmt.Sprintf("%s/repos/%s/readme?ref=%s", c.base, repo, ref)
	if err := c.get(u, &payload); err != nil {
		if errors.Is(err, errNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	if payload.Encoding != "base64" {
		return "", false, nil
	}
	raw, decErr := base64.StdEncoding.DecodeString(strings.ReplaceAll(payload.Content, "\n", ""))
	if decErr != nil {
		return "", false, fmt.Errorf("decode readme %s: %w", repo, decErr)
	}
	return string(raw), true, nil
}

// collectPinned gathers READMEs from maintained repositories as they stood before the
// cutoff, writing them to path as JSONL.
func collectPinned(want int, path string, w io.Writer) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	return c.collectPinnedInto(want, path, w)
}

// collectPinnedInto is collectPinned with the client supplied, for the same reason.
func (c *client) collectPinnedInto(want int, path string, w io.Writer) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	enc := json.NewEncoder(f)
	seen := map[string]bool{}
	written := 0
	perSlice := want/(len(pinnedStarSlices)*len(collectLanguages)) + 1

	for _, lang := range collectLanguages {
		for _, slice := range pinnedStarSlices {
			if written >= want {
				break
			}
			// No push filter here: the repository may be active, since the README is
			// read at a pre-cutoff commit rather than at its head.
			q := fmt.Sprintf("language:%s stars:%s", lang, slice)
			hits, searchErr := c.searchQuery(q, perSlice)
			if searchErr != nil {
				_, _ = fmt.Fprintf(w, "search %s %s: %v\n", lang, slice, searchErr)
			}
			if len(hits) > perSlice {
				hits = hits[:perSlice]
			}
			for _, h := range hits {
				if written >= want || seen[h.FullName] {
					continue
				}
				seen[h.FullName] = true
				sha, ok, cErr := c.lastCommitBefore(h.FullName, collectCutoff)
				if cErr != nil {
					_, _ = fmt.Fprintf(w, "commits %s: %v\n", h.FullName, cErr)
					continue
				}
				if !ok {
					continue
				}
				text, ok, rErr := c.readmeAt(h.FullName, sha)
				if rErr != nil {
					_, _ = fmt.Fprintf(w, "readme %s: %v\n", h.FullName, rErr)
					continue
				}
				if !ok || strings.TrimSpace(text) == "" {
					continue
				}
				if err := enc.Encode(Readme{
					Repo:     h.FullName,
					SHA:      sha,
					PushedAt: "pinned<" + collectCutoff,
					Stars:    h.Stars,
					Language: lang,
					Text:     text,
				}); err != nil {
					_, _ = fmt.Fprintf(w, "write %s: %v\n", h.FullName, err)
					continue
				}
				written++
				if written%25 == 0 {
					_, _ = fmt.Fprintf(w, "collected %d/%d\n", written, want)
				}
			}
		}
	}
	_, _ = fmt.Fprintf(w, "collected %d pinned READMEs into %s\n", written, path)
	return nil
}

// pinnedStarSlices partition the pinned search. The floor is higher than the push-date
// sample's because a maintained project with a real README is what this sample is for.
//
//nolint:gochecknoglobals // Immutable lookup.
var pinnedStarSlices = []string{
	"200..399", "400..799", "800..1599", "1600..3199", "3200..*",
}

// searchQuery runs one repository search and returns up to want hits.
func (c *client) searchQuery(q string, want int) ([]repoHit, error) {
	var out []repoHit
	for page := 1; page <= 10 && len(out) < want; page++ {
		u := fmt.Sprintf("%s/search/repositories?q=%s&per_page=100&page=%d&sort=stars",
			c.base, url.QueryEscape(q), page)
		var payload struct {
			Items []repoHit `json:"items"`
		}
		if err := c.get(u, &payload); err != nil {
			return out, err
		}
		if len(payload.Items) == 0 {
			break
		}
		out = append(out, payload.Items...)
		time.Sleep(c.pause)
	}
	return out, nil
}
