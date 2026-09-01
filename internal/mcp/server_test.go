package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dcadolph/slop-chop/sanitize"
)

// TestTools checks that the server advertises the four tools, each with a description a
// client's tool picker can read and an input schema that requires only the text.
func TestTools(t *testing.T) {
	t.Parallel()
	cs := newTestSession(t, NewServer(testSanitizers(), nil, nil, "test"))

	res, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	got := make(map[string]*mcpsdk.Tool, len(res.Tools))
	for _, tool := range res.Tools {
		got[tool.Name] = tool
	}

	tests := []struct {
		Name         string
		WantRequired []string
	}{{ // Test 0: chop takes the text and nothing else is mandatory.
		Name: "chop", WantRequired: []string{"text"},
	}, { // Test 1: check takes the text and nothing else is mandatory.
		Name: "check", WantRequired: []string{"text"},
	}, { // Test 2: presets takes no arguments at all.
		Name: "presets", WantRequired: nil,
	}, { // Test 3: drift takes the draft and nothing else is mandatory.
		Name: "drift", WantRequired: []string{"text"},
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			tool, ok := got[test.Name]
			if !ok {
				t.Fatalf("tool %q not advertised, have %v", test.Name, res.Tools)
			}
			if tool.Description == "" {
				t.Errorf("tool %q has no description", test.Name)
			}
			var schema struct {
				Required []string `json:"required"`
			}
			b, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatalf("marshal input schema: %v", err)
			}
			if err := json.Unmarshal(b, &schema); err != nil {
				t.Fatalf("unmarshal input schema: %v", err)
			}
			if diff := cmp.Diff(test.WantRequired, schema.Required, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("required mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestChop drives the chop tool over its happy paths and every error path, checking the
// cleaned text, the score movement, and that a bad argument comes back as a tool error the
// caller can read rather than a protocol failure.
func TestChop(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Args           map[string]any
		WantText       string
		WantErr        string
		WantFindings   int
		WantScoreDrops bool
	}{{ // Test 0: the rules swap the word and the em-dash, and the score comes down.
		Args:           map[string]any{"text": "we leverage it—daily, and we ship it"},
		WantText:       "we use it, daily, and we ship it",
		WantFindings:   2,
		WantScoreDrops: true,
	}, { // Test 1: clean text passes through untouched with no findings.
		Args:     map[string]any{"text": "we ship it"},
		WantText: "we ship it",
	}, { // Test 2: a preset named by the call adds its own rules on top.
		Args:         map[string]any{"text": "we utilize it", "presets": []any{"plain"}},
		WantText:     "we use it",
		WantFindings: 1,
	}, { // Test 3: a dialect named by the call rewrites the spelling.
		Args:         map[string]any{"text": "the colour of it", "dialect": "american"},
		WantText:     "the color of it",
		WantFindings: 1,
	}, { // Test 4: markdown code spans are left alone.
		Args:         map[string]any{"text": "we `leverage` it and we leverage it"},
		WantText:     "we `leverage` it and we use it",
		WantFindings: 1,
	}, { // Test 5: empty text is refused rather than answered with an empty chop.
		Args:    map[string]any{"text": ""},
		WantErr: "text is required",
	}, { // Test 6: an unknown preset names the ones that exist.
		Args:    map[string]any{"text": "we ship it", "presets": []any{"nope"}},
		WantErr: "unknown preset",
	}, { // Test 7: an unknown dialect is refused.
		Args:    map[string]any{"text": "we ship it", "dialect": "klingon"},
		WantErr: "unknown dialect",
	}, { // Test 8: text over the size cap is refused before any work is done.
		Args:    map[string]any{"text": strings.Repeat("a", maxTextBytes+1)},
		WantErr: "over the",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			cs := newTestSession(t, NewServer(testSanitizers(), nil, nil, "test"))
			res, err := cs.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: "chop", Arguments: test.Args})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if test.WantErr != "" {
				assertToolError(t, res, test.WantErr)
				return
			}
			if res.IsError {
				t.Fatalf("tool error: %s", textContent(t, res))
			}
			// The cleaned text is the content block, so a caller can use the reply as it is.
			if got := textContent(t, res); got != test.WantText {
				t.Errorf("text = %q, want %q", got, test.WantText)
			}
			var out chopOutput
			structured(t, res, &out)
			if out.Text != test.WantText {
				t.Errorf("structured text = %q, want %q", out.Text, test.WantText)
			}
			if len(out.Findings) != test.WantFindings {
				t.Errorf("findings = %d, want %d: %+v", len(out.Findings), test.WantFindings, out.Findings)
			}
			if out.ModelRewrite {
				t.Error("modelRewrite = true, want false when the call did not ask for it")
			}
			if test.WantScoreDrops && out.ScoreAfter >= out.Score {
				t.Errorf("score %d to %d, want the chop to bring it down", out.Score, out.ScoreAfter)
			}
		})
	}
}

// TestChopModelRewrite checks the opt-in model pass: it is off unless the call asks, a server
// with no backend refuses rather than passing the rules output off as a rewrite, the rules run
// again over whatever the model returns, and a backend failure surfaces as a tool error.
func TestChopModelRewrite(t *testing.T) {
	t.Parallel()

	// Test 0: the model pass does not run unless the call asks for it.
	t.Run("test 0", func(t *testing.T) {
		t.Parallel()
		called := false
		rw := RewriterFunc(func(context.Context, string, []string) (string, error) {
			called = true
			return "rewritten", nil
		})
		cs := newTestSession(t, NewServer(testSanitizers(), rw, nil, "test"))
		res, err := cs.CallTool(t.Context(), &mcpsdk.CallToolParams{
			Name:      "chop",
			Arguments: map[string]any{"text": "we leverage it"},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if called {
			t.Error("the model pass ran without model_rewrite")
		}
		if got := textContent(t, res); got != "we use it" {
			t.Errorf("text = %q, want the rules output", got)
		}
	})

	// Test 1: with model_rewrite the reply is re-cleaned, so a model that puts a tell back
	// cannot smuggle it past the rules, and the tone notes reach the backend.
	t.Run("test 1", func(t *testing.T) {
		t.Parallel()
		var gotText string
		var gotTone []string
		rw := RewriterFunc(func(_ context.Context, text string, tone []string) (string, error) {
			gotText, gotTone = text, tone
			return "we leverage it again", nil
		})
		cs := newTestSession(t, NewServer(testSanitizers(), rw, nil, "test"))
		res, err := cs.CallTool(t.Context(), &mcpsdk.CallToolParams{
			Name:      "chop",
			Arguments: map[string]any{"text": "we leverage it", "model_rewrite": true},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if res.IsError {
			t.Fatalf("tool error: %s", textContent(t, res))
		}
		if gotText != "we use it" {
			t.Errorf("the model saw %q, want the rules output", gotText)
		}
		if diff := cmp.Diff([]string{"dry"}, gotTone, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("tone mismatch (-want +got):\n%s", diff)
		}
		if got := textContent(t, res); got != "we use it again" {
			t.Errorf("text = %q, want the rules run again over the model reply", got)
		}
		var out chopOutput
		structured(t, res, &out)
		if !out.ModelRewrite {
			t.Error("modelRewrite = false, want true when the pass ran")
		}
	})

	// Test 2: a server with no backend refuses instead of quietly returning the rules output,
	// which would look like the model pass ran when it never did.
	t.Run("test 2", func(t *testing.T) {
		t.Parallel()
		cs := newTestSession(t, NewServer(testSanitizers(), nil, nil, "test"))
		res, err := cs.CallTool(t.Context(), &mcpsdk.CallToolParams{
			Name:      "chop",
			Arguments: map[string]any{"text": "we leverage it", "model_rewrite": true},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		assertToolError(t, res, "the model rewrite is not configured")
	})

	// Test 3: a backend failure comes back as a readable tool error.
	t.Run("test 3", func(t *testing.T) {
		t.Parallel()
		rw := RewriterFunc(func(context.Context, string, []string) (string, error) {
			return "", errors.New("no api key")
		})
		cs := newTestSession(t, NewServer(testSanitizers(), rw, nil, "test"))
		res, err := cs.CallTool(t.Context(), &mcpsdk.CallToolParams{
			Name:      "chop",
			Arguments: map[string]any{"text": "we leverage it", "model_rewrite": true},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		assertToolError(t, res, "model rewrite: no api key")
	})
}

// TestCheck drives the check tool, which reports the tells and scores the text without
// changing it.
func TestCheck(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Args         map[string]any
		WantErr      string
		WantRules    []string
		WantScoreMin int
	}{{ // Test 0: each tell is reported with the rule that matched, in text order.
		Args:         map[string]any{"text": "we leverage it—daily, and we ship it"},
		WantRules:    []string{"replace", "char:—"},
		WantScoreMin: 1,
	}, { // Test 1: clean text reports nothing and scores zero.
		Args:      map[string]any{"text": "we ship it"},
		WantRules: nil,
	}, { // Test 2: empty text is refused the same way chop refuses it.
		Args:    map[string]any{"text": ""},
		WantErr: "text is required",
	}, { // Test 3: an unknown preset is refused.
		Args:    map[string]any{"text": "we ship it", "presets": []any{"nope"}},
		WantErr: "unknown preset",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			cs := newTestSession(t, NewServer(testSanitizers(), nil, nil, "test"))
			res, err := cs.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: "check", Arguments: test.Args})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if test.WantErr != "" {
				assertToolError(t, res, test.WantErr)
				return
			}
			if res.IsError {
				t.Fatalf("tool error: %s", textContent(t, res))
			}
			var out checkOutput
			structured(t, res, &out)
			rules := make([]string, 0, len(out.Findings))
			for _, f := range out.Findings {
				rules = append(rules, f.Rule)
			}
			if diff := cmp.Diff(test.WantRules, rules, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("rules mismatch (-want +got):\n%s", diff)
			}
			if out.Score < test.WantScoreMin {
				t.Errorf("score = %d, want at least %d", out.Score, test.WantScoreMin)
			}
		})
	}
}

// TestPresets checks that the presets tool lists the packs the engine ships, so a caller can
// discover the names the other two tools accept.
func TestPresets(t *testing.T) {
	t.Parallel()
	cs := newTestSession(t, NewServer(testSanitizers(), nil, nil, "test"))
	res, err := cs.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: "presets"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool error: %s", textContent(t, res))
	}
	var out presetsOutput
	structured(t, res, &out)
	if diff := cmp.Diff(sanitize.PresetNames(), out.Presets, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("presets mismatch (-want +got):\n%s", diff)
	}
	if len(out.Presets) == 0 {
		t.Error("no presets listed, want the built-in packs")
	}
}

// TestNewServerNilSanitizers checks that building a server without a rules engine panics,
// since that is a developer error rather than a runtime condition.
func TestNewServerNilSanitizers(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("NewServer with a nil Sanitizers did not panic")
		}
	}()
	NewServer(nil, nil, nil, "test")
}

// testSanitizers builds the small fixed rules engine the tests drive, applying the presets
// and dialect a call asked for on top of it.
func testSanitizers() SanitizerFunc {
	return func(presets []string, dialect string) (*sanitize.Sanitizer, sanitize.Profile, error) {
		p := sanitize.Profile{
			CharReplace:    map[string]string{"—": ", "},
			WordReplace:    map[string]string{"leverage": "use"},
			CollapseSpaces: true,
			Tone:           []string{"dry"},
			Dialect:        sanitize.Dialect(dialect),
		}
		if len(presets) > 0 {
			merged, err := sanitize.ApplyPresets(p, presets...)
			if err != nil {
				return nil, sanitize.Profile{}, err
			}
			p = merged
		}
		s, err := sanitize.New(p)
		if err != nil {
			return nil, sanitize.Profile{}, err
		}
		return s, p, nil
	}
}

// newTestSession connects a client to srv over an in-memory transport and returns the client
// session, closing both ends when the test finishes.
func newTestSession(t *testing.T, srv *Server) *mcpsdk.ClientSession {
	t.Helper()
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	ss, err := srv.Connect(t.Context(), serverTransport)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "test"}, nil)
	cs, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// assertToolError checks that the call came back as a tool error whose message carries want.
// A bad argument must reach the caller as readable text with IsError set, not as a protocol
// failure, so the model on the other end can see what went wrong and correct itself.
func assertToolError(t *testing.T, res *mcpsdk.CallToolResult, want string) {
	t.Helper()
	if !res.IsError {
		t.Fatalf("IsError = false, want a tool error carrying %q", want)
	}
	if got := textContent(t, res); !strings.Contains(got, want) {
		t.Errorf("error = %q, want it to contain %q", got, want)
	}
}

// textContent returns the text of the first content block of a call result.
func textContent(t *testing.T, res *mcpsdk.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatal("no content in the result")
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("content = %T, want text", res.Content[0])
	}
	return text.Text
}

// structured decodes a call result's structured output into v.
func structured(t *testing.T, res *mcpsdk.CallToolResult, v any) {
	t.Helper()
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
}

// writerSample builds plain, short, first-person sentences, the register the drift tests
// treat as the user's own writing.
func writerSample(n int) string {
	var sb strings.Builder
	for i := range n {
		fmt.Fprintf(&sb, "I wrote the note on day %d and I sent it. ", i)
		if i%3 == 2 {
			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}

// testFingerprints returns a Fingerprints measured from writerSample, the seam the drift
// tests drive the tool through.
func testFingerprints(t *testing.T) FingerprintFunc {
	t.Helper()
	f, err := sanitize.NewFingerprint(writerSample(40))
	if err != nil {
		t.Fatalf("NewFingerprint: %v", err)
	}
	return func() (sanitize.Fingerprint, error) { return f, nil }
}

// TestDrift drives the drift tool: the user's own register, a machine's, and every way the
// call can fail.
func TestDrift(t *testing.T) {
	t.Parallel()
	machine := strings.Repeat("The comprehensive implementation of organizational observability "+
		"strategies demonstrates substantial operational improvements across distributed "+
		"infrastructure environments throughout the transition period. ", 8)

	tests := []struct {
		Name         string
		Args         map[string]any
		Fingerprints Fingerprints
		WantErr      string
		WantDrift    bool
		WantContent  string
	}{{ // Test 0: More of the user's own writing reads like them.
		Name: "own register", Args: map[string]any{"text": writerSample(12)},
		WantDrift: false, WantContent: "reads like the user",
	}, { // Test 1: A long, formal register reads unlike them, and the report says how.
		Name: "machine register", Args: map[string]any{"text": machine},
		WantDrift: true, WantContent: "reads unlike the user",
	}, { // Test 2: A note is too short to carry a reading.
		Name: "too short", Args: map[string]any{"text": "I wrote this one."},
		WantErr: "not enough text",
	}, { // Test 3: Empty text is refused before any measuring.
		Name: "empty", Args: map[string]any{"text": ""}, WantErr: "text is required",
	}, { // Test 4: Text over the cap is refused.
		Name: "too big", Args: map[string]any{"text": strings.Repeat("a", maxTextBytes+1)},
		WantErr: "over the",
	}, { // Test 5: A server wired without a voice says so rather than passing everything.
		Name: "no voice", Args: map[string]any{"text": writerSample(12)},
		Fingerprints: FingerprintFunc(func() (sanitize.Fingerprint, error) {
			return sanitize.Fingerprint{}, errors.New("no fingerprint in voice.json")
		}),
		WantErr: "no fingerprint",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			fps := test.Fingerprints
			if fps == nil {
				fps = testFingerprints(t)
			}
			cs := newTestSession(t, NewServer(testSanitizers(), nil, fps, "test"))
			res, err := cs.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: "drift", Arguments: test.Args})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if test.WantErr != "" {
				assertToolError(t, res, test.WantErr)
				return
			}
			if res.IsError {
				t.Fatalf("tool error: %s", textContent(t, res))
			}
			if got := textContent(t, res); !strings.Contains(got, test.WantContent) {
				t.Errorf("content = %q, want it to hold %q", got, test.WantContent)
			}
			var out driftOutput
			b, err := json.Marshal(res.StructuredContent)
			if err != nil {
				t.Fatalf("marshal structured content: %v", err)
			}
			if err := json.Unmarshal(b, &out); err != nil {
				t.Fatalf("unmarshal structured content: %v", err)
			}
			if out.ReadsLikeYou == test.WantDrift {
				t.Errorf("readsLikeYou = %v with %d drifted traits", out.ReadsLikeYou, len(out.Drift))
			}
			if test.WantDrift && len(out.Drift) == 0 {
				t.Error("drift reported none on a machine register")
			}
			if out.Traits == 0 {
				t.Error("traits = 0, want the number measured")
			}
		})
	}
}

// TestDriftNoFingerprints checks that a server built without a Fingerprints refuses drift
// instead of panicking on a nil provider.
func TestDriftNoFingerprints(t *testing.T) {
	t.Parallel()
	cs := newTestSession(t, NewServer(testSanitizers(), nil, nil, "test"))
	res, err := cs.CallTool(t.Context(), &mcpsdk.CallToolParams{
		Name:      "drift",
		Arguments: map[string]any{"text": writerSample(12)},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	assertToolError(t, res, "needs a voice")
}
