//go:build integration

package rewrite

import (
	"os"
	"strings"
	"testing"
)

// This test makes real calls to an OpenAI-compatible API. The integration build tag keeps
// it out of a normal build, and it skips unless a key or a base URL is set, so it runs only
// when asked for. Against hosted OpenAI:
//
//	OPENAI_API_KEY=sk-... go test -tags=integration ./internal/rewrite/ -run OpenAILive -v
//
// Against a local server like Ollama, which needs no key:
//
//	OPENAI_BASE_URL=http://localhost:11434/v1 OPENAI_MODEL=llama3.1 \
//	  go test -tags=integration ./internal/rewrite/ -run OpenAILive -v

// requireOpenAI skips the test unless a key or a base URL is set. A local server takes the
// base URL with no key, so either one is enough to opt in.
func requireOpenAI(t *testing.T) {
	t.Helper()
	if os.Getenv("OPENAI_API_KEY") == "" && os.Getenv("OPENAI_BASE_URL") == "" {
		t.Skip("set OPENAI_API_KEY or OPENAI_BASE_URL to run the live OpenAI test")
	}
}

// TestRewriteOpenAILive rewrites a slop sentence against a real OpenAI-compatible model. It
// proves the request and response shape against the live API rather than a stub: a non-empty
// reply that differs from the input. The base URL and model come from the environment so the
// same test covers hosted OpenAI and a local server.
func TestRewriteOpenAILive(t *testing.T) {
	requireOpenAI(t)
	in := "In summary, we leveraged a comprehensive—and robust—solution; it works."
	c := NewOpenAICompleter(os.Getenv("OPENAI_MODEL"), os.Getenv("OPENAI_BASE_URL"))
	out, err := New(c).Rewrite(liveContext(t), in)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	if out == "" {
		t.Fatal("Rewrite returned empty text")
	}
	if out == in {
		t.Errorf("Rewrite returned the input unchanged: %q", out)
	}
	if strings.Contains(out, "—") {
		t.Logf("note: the rewrite kept an em-dash, which the rules pass would clean: %q", out)
	}
	t.Logf("live openai rewrite:\n in:  %q\n out: %q", in, out)
}
