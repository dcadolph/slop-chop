//go:build integration

package rewrite

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// These tests make real, paid calls to the Anthropic API. The integration build tag keeps
// them out of a normal build, and they skip when ANTHROPIC_API_KEY is unset, so they run
// only when asked for. Run them with:
//
//	ANTHROPIC_API_KEY=sk-... go test -tags=integration ./internal/rewrite/ -run Live -v

// requireKey skips the test unless a real API key is present.
func requireKey(t *testing.T) {
	t.Helper()
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("set ANTHROPIC_API_KEY to run the live integration test")
	}
}

// liveContext returns a context with a timeout generous enough for one model call.
func liveContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// TestRewriteLive rewrites a slop sentence against the real model. It proves the request
// and the response shape against the live API rather than a stub: a non-empty reply that
// differs from the input. Whether the model dropped every em-dash is logged, not asserted,
// since the rules pass cleans any it leaves.
func TestRewriteLive(t *testing.T) {
	requireKey(t)
	in := "In summary, we leveraged a comprehensive—and robust—solution; it works."
	out, err := New(NewAnthropicCompleter(DefaultModel)).Rewrite(liveContext(t), in)
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
	t.Logf("live rewrite:\n in:  %q\n out: %q", in, out)
}

// TestJudgeLive checks the meaning check against the real model on an obvious pair: a
// faithful paraphrase passes and a flipped number fails. Both are clear enough that a
// capable model is reliable, so this proves the judge parses a real verdict correctly.
func TestJudgeLive(t *testing.T) {
	requireKey(t)
	j := NewJudge(NewAnthropicCompleter(DefaultModel))
	const orig = "Revenue rose 10 percent in the third quarter."

	faithful, err := j.Judge(liveContext(t), orig, "In Q3, revenue was up ten percent.")
	if err != nil {
		t.Fatalf("Judge faithful: %v", err)
	}
	if !faithful.Faithful {
		t.Errorf("a faithful paraphrase was judged unfaithful: %+v", faithful)
	}

	drifted, err := j.Judge(liveContext(t), orig, "Revenue fell 40 percent in the third quarter.")
	if err != nil {
		t.Fatalf("Judge drifted: %v", err)
	}
	if drifted.Faithful {
		t.Errorf("a flipped number was judged faithful: %+v", drifted)
	}
}
