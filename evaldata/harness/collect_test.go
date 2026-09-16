package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// testClient returns a collection client pointed at srv with no pauses, so the retry and
// paging paths run at test speed rather than at the API's.
func testClient(srv *httptest.Server) *client {
	return &client{token: "test-token", http: srv.Client(), base: srv.URL, pause: 0}
}

// encodeReadme renders a README payload the way the contents endpoint does.
func encodeReadme(body, sha string) string {
	return fmt.Sprintf(`{"content":%q,"encoding":"base64","sha":%q}`,
		base64.StdEncoding.EncodeToString([]byte(body)), sha)
}

// TestClientGet checks the one place every collection call goes through: a 404 has to be
// separable from a real failure, or an absent README and a rate limit look the same and
// the sample quietly shrinks.
func TestClientGet(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name     string
		Status   int
		Body     string
		WantErr  error
		WantText string
	}{
		{Name: "ok", Status: 200, Body: `{"sha":"abc"}`, WantText: "abc"},  // Test 0.
		{Name: "not found", Status: 404, Body: `{}`, WantErr: errNotFound}, // Test 1.
		{Name: "server error", Status: 500, Body: `{}`, WantErr: nil},      // Test 2.
		{Name: "bad json", Status: 200, Body: `{oops`, WantErr: nil},       // Test 3.
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
					t.Errorf("Authorization = %q, want the bearer token", got)
				}
				w.WriteHeader(test.Status)
				_, _ = io.WriteString(w, test.Body)
			}))
			defer srv.Close()

			var payload struct {
				SHA string `json:"sha"`
			}
			err := testClient(srv).get(srv.URL, &payload)
			switch {
			case test.WantErr != nil:
				if !errors.Is(err, test.WantErr) {
					t.Errorf("err = %v, want %v", err, test.WantErr)
				}
			case test.WantText != "":
				if err != nil {
					t.Fatalf("get: %v", err)
				}
				if payload.SHA != test.WantText {
					t.Errorf("sha = %q, want %q", payload.SHA, test.WantText)
				}
			default:
				if err == nil {
					t.Errorf("err = nil, want a failure")
				}
			}
		})
	}
}

// TestClientGetRetriesRateLimit checks that a rate-limited reply is retried once and then
// succeeds, since a run of a thousand meets the secondary limit at least once.
func TestClientGetRetriesRateLimit(t *testing.T) {
	t.Parallel()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = io.WriteString(w, `{"sha":"second"}`)
	}))
	defer srv.Close()

	var payload struct {
		SHA string `json:"sha"`
	}
	if err := testClient(srv).get(srv.URL, &payload); err != nil {
		t.Fatalf("get: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
	if payload.SHA != "second" {
		t.Errorf("sha = %q, want the retried reply", payload.SHA)
	}
}

// TestReadme checks the README fetch, including the case that must not be an error: a
// repository that simply has no README is a sample to skip.
func TestReadme(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name     string
		Status   int
		Body     string
		WantOK   bool
		WantText string
		WantErr  bool
	}{{ // Test 0: A normal README.
		Name: "present", Status: 200, Body: encodeReadme("# Title\n\nBody.", "sha1"),
		WantOK: true, WantText: "# Title\n\nBody.",
	}, { // Test 1: No README is not a failure.
		Name: "absent", Status: 404, Body: `{}`, WantOK: false,
	}, { // Test 2: An unexpected encoding is skipped rather than guessed at.
		Name: "other encoding", Status: 200, Body: `{"content":"x","encoding":"utf-8"}`, WantOK: false,
	}, { // Test 3: A real failure must surface, not read as an absent file.
		Name: "server error", Status: 500, Body: `{}`, WantOK: false, WantErr: true,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.Status)
				_, _ = io.WriteString(w, test.Body)
			}))
			defer srv.Close()

			text, sha, ok, err := testClient(srv).readme("owner/repo")
			if gotErr := err != nil; gotErr != test.WantErr {
				t.Fatalf("err = %v, want failure %v", err, test.WantErr)
			}
			if ok != test.WantOK {
				t.Errorf("ok = %v, want %v", ok, test.WantOK)
			}
			if test.WantOK {
				if text != test.WantText {
					t.Errorf("text = %q, want %q", text, test.WantText)
				}
				if sha != "sha1" {
					t.Errorf("sha = %q, want sha1", sha)
				}
			}
		})
	}
}

// TestLastCommitBefore checks the pinned sample's anchor: the newest commit before the
// cutoff, and the honest false when a repository has none that early.
func TestLastCommitBefore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name    string
		Body    string
		WantSHA string
		WantOK  bool
	}{
		{Name: "found", Body: `[{"sha":"deadbeef"}]`, WantSHA: "deadbeef", WantOK: true}, // Test 0.
		{Name: "none that early", Body: `[]`, WantOK: false},                             // Test 1.
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.URL.RawQuery, "until=") {
					t.Errorf("query %q is missing the cutoff", r.URL.RawQuery)
				}
				_, _ = io.WriteString(w, test.Body)
			}))
			defer srv.Close()

			sha, ok, err := testClient(srv).lastCommitBefore("owner/repo", collectCutoff)
			if err != nil {
				t.Fatalf("lastCommitBefore: %v", err)
			}
			if ok != test.WantOK || sha != test.WantSHA {
				t.Errorf("= %q, %v, want %q, %v", sha, ok, test.WantSHA, test.WantOK)
			}
		})
	}
}

// TestReadmeAt checks that the pinned fetch asks for the pinned ref, since reading the
// head instead would silently collect post-cutoff prose and void the guarantee.
func TestReadmeAt(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ref") != "pinnedsha" {
			t.Errorf("ref = %q, want pinnedsha", r.URL.Query().Get("ref"))
		}
		_, _ = io.WriteString(w, encodeReadme("old prose", "x"))
	}))
	defer srv.Close()

	text, ok, err := testClient(srv).readmeAt("owner/repo", "pinnedsha")
	if err != nil || !ok {
		t.Fatalf("readmeAt = %v, %v, want ok", err, ok)
	}
	if text != "old prose" {
		t.Errorf("text = %q, want the pinned body", text)
	}
}

// TestSearchQueryPages checks that paging stops on an empty page rather than walking to
// the cap, which would spend the search budget on nothing.
func TestSearchQueryPages(t *testing.T) {
	t.Parallel()
	pages := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		pages++
		if pages == 1 {
			_, _ = io.WriteString(w, `{"items":[{"full_name":"a/a","pushed_at":"2020-01-01T00:00:00Z","stargazers_count":60}]}`)
			return
		}
		_, _ = io.WriteString(w, `{"items":[]}`)
	}))
	defer srv.Close()

	hits, err := testClient(srv).searchQuery("language:Go", 50)
	if err != nil {
		t.Fatalf("searchQuery: %v", err)
	}
	if diff := cmp.Diff(1, len(hits), cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("hit count mismatch (-want +got):\n%s", diff)
	}
	if pages != 2 {
		t.Errorf("pages = %d, want 2: one with items and one empty", pages)
	}
}

// TestWriteHits checks the three guards that keep the sample honest: a repository is
// written once, a push at or after the cutoff is refused even though the search already
// filtered it, and the global budget stops the run.
func TestWriteHits(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, encodeReadme("some prose", "sha"))
	}))
	defer srv.Close()

	var out strings.Builder
	enc := json.NewEncoder(&out)
	seen := map[string]bool{}
	hits := []repoHit{
		{FullName: "a/a", PushedAt: "2020-01-01T00:00:00Z"},
		{FullName: "a/a", PushedAt: "2020-01-01T00:00:00Z"}, // duplicate
		{FullName: "b/b", PushedAt: "2023-06-01T00:00:00Z"}, // after the cutoff
		{FullName: "c/c", PushedAt: "2019-01-01T00:00:00Z"},
	}
	written := testClient(srv).writeHits("Go", hits, seen, enc, io.Discard, 10, 0)
	if written != 2 {
		t.Errorf("written = %d, want 2: the duplicate and the post-cutoff repo are refused", written)
	}
	if strings.Contains(out.String(), "b/b") {
		t.Errorf("a repository pushed after the cutoff was written:\n%s", out.String())
	}
	if !strings.Contains(out.String(), `"language":"Go"`) {
		t.Errorf("the ecosystem was not recorded:\n%s", out.String())
	}

	// The budget stops the run even when hits remain.
	capped := testClient(srv).writeHits("Go", hits, map[string]bool{}, json.NewEncoder(io.Discard), io.Discard, 1, 0)
	if capped != 1 {
		t.Errorf("capped = %d, want the budget to stop at 1", capped)
	}
}

// TestGithubToken checks that the environment is preferred, so a run in CI never depends
// on a developer's local gh login.
func TestGithubToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "from-env")
	got, err := githubToken()
	if err != nil {
		t.Fatalf("githubToken: %v", err)
	}
	if got != "from-env" {
		t.Errorf("token = %q, want from-env", got)
	}

	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "from-gh-env")
	got, err = githubToken()
	if err != nil {
		t.Fatalf("githubToken: %v", err)
	}
	if got != "from-gh-env" {
		t.Errorf("token = %q, want from-gh-env", got)
	}
}

// TestCollectWritesJSONL checks the whole push-date path end to end against a local
// server, including that the file it writes reads back as the samples the report scores.
func TestCollectWritesJSONL(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/search/") {
			_, _ = io.WriteString(w, `{"items":[{"full_name":"owner/one","pushed_at":"2020-05-05T00:00:00Z","stargazers_count":99}]}`)
			return
		}
		_, _ = io.WriteString(w, encodeReadme("# One\n\nReal prose here.", "sha9"))
	}))
	defer srv.Close()

	path := t.TempDir() + "/out.jsonl"
	c := testClient(srv)
	if err := c.collectInto(1, path, io.Discard); err != nil {
		t.Fatalf("collectInto: %v", err)
	}
	got, err := readLines[Readme](path)
	if err != nil {
		t.Fatalf("readLines: %v", err)
	}
	want := []Readme{{
		Repo: "owner/one", SHA: "sha9", PushedAt: "2020-05-05", Stars: 99,
		Language: collectLanguages[0], Text: "# One\n\nReal prose here.",
	}}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("collected samples mismatch (-want +got):\n%s", diff)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("output file: %v", err)
	}
}

// TestCollectPinnedInto checks the pinned path end to end: the README must be read at the
// repository's last pre-cutoff commit, not at its head, since reading the head would
// collect prose written after the models existed and void the guarantee.
func TestCollectPinnedInto(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/search/"):
			_, _ = io.WriteString(w, `{"items":[{"full_name":"owner/live","pushed_at":"2026-01-01T00:00:00Z","stargazers_count":900}]}`)
		case strings.Contains(r.URL.Path, "/commits"):
			_, _ = io.WriteString(w, `[{"sha":"oldsha"}]`)
		default:
			if r.URL.Query().Get("ref") != "oldsha" {
				t.Errorf("ref = %q, want the pinned commit", r.URL.Query().Get("ref"))
			}
			_, _ = io.WriteString(w, encodeReadme("Prose from before the cutoff.", "x"))
		}
	}))
	defer srv.Close()

	path := t.TempDir() + "/pinned.jsonl"
	if err := testClient(srv).collectPinnedInto(1, path, io.Discard); err != nil {
		t.Fatalf("collectPinnedInto: %v", err)
	}
	got, err := readLines[Readme](path)
	if err != nil {
		t.Fatalf("readLines: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("collected %d samples, want 1", len(got))
	}
	// The repository is still active, which is the whole point of the pinned sample.
	if got[0].PushedAt != "pinned<"+collectCutoff {
		t.Errorf("pushedAt = %q, want the pinned marker", got[0].PushedAt)
	}
	if got[0].SHA != "oldsha" || got[0].Text != "Prose from before the cutoff." {
		t.Errorf("sample = %+v, want the pre-cutoff README", got[0])
	}
	if provenanceOf(got) == "" || !strings.Contains(provenanceOf(got), "maintained") {
		t.Errorf("provenance = %q, want it to say the projects are maintained", provenanceOf(got))
	}
}

// TestReadmeAtErrors checks that a real failure surfaces from the pinned fetch while an
// absent README stays a plain skip.
func TestReadmeAtErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name    string
		Status  int
		WantErr bool
	}{
		{Name: "absent", Status: 404, WantErr: false},      // Test 0: A skip, not a failure.
		{Name: "server error", Status: 500, WantErr: true}, // Test 1: Must not read as absent.
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.Status)
				_, _ = io.WriteString(w, `{}`)
			}))
			defer srv.Close()
			_, ok, err := testClient(srv).readmeAt("owner/repo", "ref")
			if gotErr := err != nil; gotErr != test.WantErr {
				t.Errorf("err = %v, want failure %v", err, test.WantErr)
			}
			if ok {
				t.Errorf("ok = true, want false")
			}
		})
	}
}

// TestRunPre2022 checks the command path that reads a corpus file and writes the report.
func TestRunPre2022(t *testing.T) {
	t.Parallel()
	path := t.TempDir() + "/samples.jsonl"
	body := strings.TrimSpace(strings.Repeat("the survey crew set a benchmark on the ridge that morning ", 30))
	line, err := json.Marshal(Readme{Repo: "a/a", PushedAt: "2020-01-01", Language: "Go", Text: body})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, append(line, '\n'), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	var out strings.Builder
	if err := runPre2022(path, &out); err != nil {
		t.Fatalf("runPre2022: %v", err)
	}
	for _, want := range []string{"false positive measurement", "samples scored:   1", "Go 1"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report is missing %q:\n%s", want, out.String())
		}
	}
}
