package rewrite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestRewrite checks that Rewrite passes the text to the completer and trims the reply.
func TestRewrite(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In         string
		Reply      string
		WantResult string
		Want       error
	}{{ // Test 0: Reply is returned trimmed.
		In: "dirty", Reply: "  clean text\n", WantResult: "clean text", Want: nil,
	}, { // Test 1: Completer error is wrapped.
		In: "dirty", Reply: "", WantResult: "", Want: errBoom,
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			var gotUser string
			c := CompleterFunc(func(_ context.Context, _, user string) (string, error) {
				gotUser = user
				if test.Want != nil {
					return "", test.Want
				}
				return test.Reply, nil
			})
			got, err := New(c).Rewrite(t.Context(), test.In)
			if !errors.Is(err, test.Want) {
				t.Fatalf("err = %v, want %v", err, test.Want)
			}
			if test.Want != nil {
				return
			}
			if gotUser != test.In {
				t.Errorf("user prompt = %q, want %q", gotUser, test.In)
			}
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestBuildSystemTone checks that tone notes land in the system prompt.
func TestBuildSystemTone(t *testing.T) {
	t.Parallel()
	got := buildSystem([]string{"dry and direct"}, nil)
	if !strings.Contains(got, "dry and direct") {
		t.Errorf("system prompt missing tone note:\n%s", got)
	}
	if !strings.Contains(got, "em-dash") {
		t.Errorf("system prompt missing core instruction:\n%s", got)
	}
}

// TestBuildSystemFeedback checks that feedback notes land in the system prompt so a retry
// can preserve the flagged facts.
func TestBuildSystemFeedback(t *testing.T) {
	t.Parallel()
	got := buildSystem(nil, []string{`keep "99.9%", do not drop it (figure)`})
	if !strings.Contains(got, "99.9%") {
		t.Errorf("system prompt missing feedback note:\n%s", got)
	}
	if !strings.Contains(got, "changed the meaning") {
		t.Errorf("system prompt missing feedback preamble:\n%s", got)
	}
}

// TestNewNilPanics checks that New panics on a nil completer.
func TestNewNilPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("New(nil): want panic")
		}
	}()
	New(nil)
}

// TestCompleteResponses checks how Complete handles API replies: success, HTTP errors,
// truncation, and multi-block content.
func TestCompleteResponses(t *testing.T) {
	tests := []struct {
		Body       string
		WantResult string
		WantErrSub string
		Status     int
	}{{ // Test 0: A finished reply returns its text.
		Status: 200, Body: `{"content":[{"type":"text","text":"clean"}],"stop_reason":"end_turn"}`,
		WantResult: "clean",
	}, { // Test 1: A non-200 status is an error carrying the status.
		Status: 500, Body: `boom`, WantErrSub: "500",
	}, { // Test 2: A reply cut off at the token cap is an error, not truncated text.
		Status: 200, Body: `{"content":[{"type":"text","text":"half"}],"stop_reason":"max_tokens"}`,
		WantErrSub: "truncated",
	}, { // Test 3: A safety refusal is a clear error, not an unexpected one.
		Status: 200, Body: `{"content":[],"stop_reason":"refusal"}`, WantErrSub: "declined",
	}, { // Test 4: Any other stop reason is reported as unexpected.
		Status: 200, Body: `{"content":[{"type":"text","text":"x"}],"stop_reason":"pause_turn"}`,
		WantErrSub: "unexpected",
	}, { // Test 5: Text blocks concatenate and non-text blocks are skipped.
		Status: 200,
		Body: `{"content":[{"type":"text","text":"a"},{"type":"thinking","text":"x"},` +
			`{"type":"text","text":"b"}],"stop_reason":"end_turn"}`,
		WantResult: "ab",
	}, { // Test 6: A finished reply with no text content is an error, not empty output.
		Status: 200, Body: `{"content":[],"stop_reason":"end_turn"}`,
		WantErrSub: "no text content",
	}, { // Test 7: A finished reply with only non-text blocks is an error too.
		Status: 200, Body: `{"content":[{"type":"thinking","text":"x"}],"stop_reason":"end_turn"}`,
		WantErrSub: "no text content",
	}, { // Test 8: A 200 with a body that is not JSON is a decode error.
		Status: 200, Body: `not json`, WantErrSub: "decode reply",
	}}

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.Status)
				_, _ = io.WriteString(w, test.Body)
			}))
			defer srv.Close()
			c := &anthropicCompleter{model: "m", endpoint: srv.URL, client: srv.Client()}
			got, err := c.Complete(t.Context(), "sys", "user")
			if test.WantErrSub != "" {
				if err == nil || !strings.Contains(err.Error(), test.WantErrSub) {
					t.Fatalf("err = %v, want substring %q", err, test.WantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("Complete: %v", err)
			}
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestCompleteRequest checks the wire request: headers and body fields.
func TestCompleteRequest(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	var gotKey, gotVersion string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn"}`)
	}))
	defer srv.Close()

	c := &anthropicCompleter{model: "model-x", endpoint: srv.URL, client: srv.Client()}
	if _, err := c.Complete(t.Context(), "sys prompt", "user text"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if gotKey != "test-key" {
		t.Errorf("x-api-key = %q", gotKey)
	}
	if gotVersion != anthropicVersion {
		t.Errorf("anthropic-version = %q", gotVersion)
	}
	var req messagesRequest
	if err := json.Unmarshal(gotBody, &req); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if req.Model != "model-x" || req.System != "sys prompt" ||
		len(req.Messages) != 1 || req.Messages[0].Content != "user text" {
		t.Errorf("request = %+v", req)
	}
}

// TestCompleteNoKey checks that a missing API key is an error before any request.
func TestCompleteNoKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	c := &anthropicCompleter{model: "m", endpoint: "http://127.0.0.1:0", client: http.DefaultClient}
	if _, err := c.Complete(t.Context(), "s", "u"); err == nil {
		t.Error("Complete: want error when key is missing")
	}
}

// TestNewAnthropicCompleterDefaults checks that an empty model falls back to
// DefaultModel.
func TestNewAnthropicCompleterDefaults(t *testing.T) {
	t.Parallel()
	c, ok := NewAnthropicCompleter("").(*anthropicCompleter)
	if !ok {
		t.Fatal("NewAnthropicCompleter: want *anthropicCompleter")
	}
	if c.model != DefaultModel {
		t.Errorf("model = %q, want %q", c.model, DefaultModel)
	}
}

// errBoom is a sentinel completer error for tests.
var errBoom = errors.New("boom")
