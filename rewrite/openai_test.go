package rewrite

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestOpenAICompleter checks the Chat Completions request and reply handling, including the
// finish reasons that must be treated as errors rather than truncated text.
//
//nolint:funlen // Table test with a handler.
func TestOpenAICompleter(t *testing.T) {
	// t.Setenv forbids t.Parallel, so this test runs serially with a fixed key.
	t.Setenv("OPENAI_API_KEY", "test-key")

	tests := []struct {
		Name         string
		Content      string
		FinishReason string
		Status       int
		NoChoices    bool
		WantResult   string
		WantErr      string
	}{
		{Name: "ok", Content: "clean text", FinishReason: "stop", Status: 200, WantResult: "clean text"},
		{Name: "empty finish reason ok", Content: "clean", FinishReason: "", Status: 200, WantResult: "clean"},
		{Name: "length is error", Content: "half", FinishReason: "length", Status: 200, WantErr: "token cap"},
		{Name: "filter is error", Content: "", FinishReason: "content_filter", Status: 200, WantErr: "declined"},
		{Name: "empty content is error", Content: "  ", FinishReason: "stop", Status: 200, WantErr: "no text content"},
		{Name: "non-200 is error", Content: "", FinishReason: "", Status: 500, WantErr: "openai api"},
		{Name: "no choices is error", NoChoices: true, Status: 200, WantErr: "no choices"},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var gotAuth, gotModel string
			var gotRoles []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("authorization")
				var req chatRequest
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &req)
				gotModel = req.Model
				for _, m := range req.Messages {
					gotRoles = append(gotRoles, m.Role)
				}
				w.WriteHeader(test.Status)
				resp := chatResponse{Choices: []chatChoice{{
					Message:      chatMessage{Role: "assistant", Content: test.Content},
					FinishReason: test.FinishReason,
				}}}
				if test.NoChoices {
					resp.Choices = nil
				}
				_ = json.NewEncoder(w).Encode(resp)
			}))
			defer srv.Close()

			c := NewOpenAICompleter("gpt-test", srv.URL)
			got, err := c.Complete(t.Context(), "sys", "user")

			if test.WantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.WantErr) {
					t.Fatalf("err = %v, want containing %q", err, test.WantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("result mismatch (-want +got):\n%s", diff)
			}
			if gotAuth != "Bearer test-key" {
				t.Errorf("authorization = %q, want %q", gotAuth, "Bearer test-key")
			}
			if gotModel != "gpt-test" {
				t.Errorf("model = %q, want gpt-test", gotModel)
			}
			if diff := cmp.Diff([]string{"system", "user"}, gotRoles); diff != "" {
				t.Errorf("roles mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestOpenAILocalNoKey checks that a non-default base URL runs without a key and sends no
// Authorization header, the path a local server like Ollama takes.
func TestOpenAILocalNoKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	sawAuth := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("authorization") != ""
		_ = json.NewEncoder(w).Encode(chatResponse{Choices: []chatChoice{{
			Message: chatMessage{Content: "local reply"}, FinishReason: "stop",
		}}})
	}))
	defer srv.Close()

	got, err := NewOpenAICompleter("", srv.URL).Complete(t.Context(), "sys", "user")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "local reply" {
		t.Errorf("result = %q, want %q", got, "local reply")
	}
	if sawAuth {
		t.Error("Authorization header sent with an empty key")
	}
}

// TestOpenAIKeyRequiredOnDefault checks that the hosted OpenAI endpoint still demands a key.
func TestOpenAIKeyRequiredOnDefault(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	_, err := NewOpenAICompleter("", "").Complete(t.Context(), "sys", "user")
	if err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY is not set") {
		t.Fatalf("err = %v, want missing key error", err)
	}
}

// TestNewCompleter checks the provider factory picks a backend and rejects unknown names.
func TestNewCompleter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Provider Provider
		WantErr  bool
	}{
		{Provider: ProviderAnthropic},
		{Provider: ProviderOpenAI},
		{Provider: ""},
		{Provider: "grok", WantErr: true},
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			c, err := NewCompleter(test.Provider, "", "")
			if test.WantErr {
				if err == nil {
					t.Fatalf("err = nil, want unknown provider error")
				}
				return
			}
			if err != nil || c == nil {
				t.Fatalf("NewCompleter(%q) = %v, %v", test.Provider, c, err)
			}
		})
	}
}

// interface guard so the completer keeps satisfying Completer.
var _ Completer = (*openAICompleter)(nil)
