// Package rewrite is the optional Layer 2 pass. It hands text to a model to do what
// the deterministic rules cannot, like reworking a sentence to drop a semicolon or
// bending the text toward a chosen voice.
package rewrite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dcadolph/slop-chop/internal/rewrite/prompt"
)

// DefaultModel is the model used when none is set.
const DefaultModel = "claude-opus-4-8"

// Completer sends a system prompt and a user prompt to a model and returns the text of
// the reply.
type Completer interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// CompleterFunc adapts a function to the Completer interface.
type CompleterFunc func(ctx context.Context, system, user string) (string, error)

// Complete calls the wrapped function.
func (f CompleterFunc) Complete(ctx context.Context, system, user string) (string, error) {
	return f(ctx, system, user)
}

// Rewriter rewrites text through a Completer using a built system prompt.
type Rewriter struct {
	// completer is the model backend.
	completer Completer
	// tone holds optional notes on the voice to aim for.
	tone []string
}

// New returns a Rewriter. It panics if completer is nil, since that is a programming
// error in this internal package.
func New(completer Completer, tone ...string) *Rewriter {
	if completer == nil {
		panic("rewrite.New: Completer required")
	}
	return &Rewriter{completer: completer, tone: tone}
}

// Rewrite returns the text rewritten to read like a person wrote it. Any feedback notes
// name facts a prior attempt changed, so the model can preserve them this time.
func (r *Rewriter) Rewrite(ctx context.Context, text string, feedback ...string) (string, error) {
	out, err := r.completer.Complete(ctx, buildSystem(r.tone, feedback), text)
	if err != nil {
		return "", fmt.Errorf("rewrite: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// buildSystem assembles the instruction that tells the model how to clean the text. Any
// feedback notes are appended so a retry keeps the facts a prior attempt changed. The
// text lives in the prompt package so the WASM build sends the same instruction.
func buildSystem(tone, feedback []string) string {
	return prompt.System(tone, feedback)
}

// Anthropic Messages API constants.
const (
	// anthropicEndpoint is the Messages API URL.
	anthropicEndpoint = "https://api.anthropic.com/v1/messages"
	// anthropicVersion pins the API version so the wire format does not drift.
	anthropicVersion = "2023-06-01"
	// maxTokens caps the length of the rewrite.
	maxTokens = 16000
	// requestTimeout bounds one API call. A rewrite of a large file can take minutes,
	// so the cap is generous, but a hung connection no longer blocks forever.
	requestTimeout = 10 * time.Minute
	// maxReplyBytes caps how much of a reply body is read into memory.
	maxReplyBytes = 10 << 20
)

// anthropicCompleter calls the Anthropic Messages API over HTTP.
type anthropicCompleter struct {
	// model is the model id sent with each request.
	model string
	// endpoint is the Messages API URL. Tests point it at a local server.
	endpoint string
	// client is the HTTP client requests go through.
	client *http.Client
}

// NewAnthropicCompleter returns a Completer that calls the Anthropic Messages API over
// HTTP. It reads the API key from the ANTHROPIC_API_KEY environment variable. Talking to
// the API directly keeps the deterministic core free of the Anthropic SDK and its
// dependency tree, so the default binary stays small.
func NewAnthropicCompleter(model string) Completer {
	if model == "" {
		model = DefaultModel
	}
	return &anthropicCompleter{
		model:    model,
		endpoint: anthropicEndpoint,
		client:   &http.Client{Timeout: requestTimeout},
	}
}

// Complete sends one Messages API request and returns the text of the reply. A reply
// that stopped for any reason other than finishing, like running into the token cap, is
// an error rather than silently truncated text.
func (c *anthropicCompleter) Complete(ctx context.Context, system, user string) (string, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY is not set")
	}

	body, err := json.Marshal(messagesRequest{
		Model:     c.model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  []message{{Role: "user", Content: user}},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	reply, err := io.ReadAll(io.LimitReader(resp.Body, maxReplyBytes))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic api: %s: %s", resp.Status, strings.TrimSpace(string(reply)))
	}

	var out messagesResponse
	if err := json.Unmarshal(reply, &out); err != nil {
		return "", fmt.Errorf("anthropic api: decode reply: %w", err)
	}
	switch out.StopReason {
	case "end_turn":
	case "max_tokens":
		return "", fmt.Errorf("anthropic api: reply hit the %d token cap and is truncated", maxTokens)
	case "refusal":
		// A safety classifier can decline the request with a 200 and an empty reply.
		return "", fmt.Errorf("anthropic api: the model declined to rewrite the text")
	default:
		return "", fmt.Errorf("anthropic api: unexpected stop_reason %q", out.StopReason)
	}
	var b strings.Builder
	for _, block := range out.Content {
		if block.Type == "text" {
			b.WriteString(block.Text)
		}
	}
	// An end_turn reply with no text would overwrite the input with nothing under
	// --write, so treat it as an error rather than silent data loss.
	if b.Len() == 0 {
		return "", fmt.Errorf("anthropic api: reply had no text content")
	}
	return b.String(), nil
}

// messagesRequest is the POST body for the Anthropic Messages API.
type messagesRequest struct {
	// Model is the model id.
	Model string `json:"model"`
	// MaxTokens caps the reply length.
	MaxTokens int `json:"max_tokens"`
	// System is the system prompt.
	System string `json:"system"`
	// Messages is the conversation, one user turn here.
	Messages []message `json:"messages"`
}

// message is one turn in a Messages API request.
type message struct {
	// Role is user or assistant.
	Role string `json:"role"`
	// Content is the turn text.
	Content string `json:"content"`
}

// messagesResponse is the part of the Messages API reply the rewriter reads.
type messagesResponse struct {
	// Content is the model's reply, a list of blocks.
	Content []contentBlock `json:"content"`
	// StopReason says why the model stopped. Anything but end_turn means the reply is
	// not the whole rewrite.
	StopReason string `json:"stop_reason"`
}

// contentBlock is one block of the model's reply.
type contentBlock struct {
	// Type is the block kind, text for prose.
	Type string `json:"type"`
	// Text is the block text.
	Text string `json:"text"`
}
