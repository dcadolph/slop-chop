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
)

// OpenAI-compatible Chat Completions constants.
const (
	// DefaultOpenAIModel is the model used for --provider openai when none is set.
	DefaultOpenAIModel = "gpt-4o"
	// DefaultOpenAIBaseURL is the OpenAI Chat Completions base. Any OpenAI-compatible
	// server, like a local Ollama or a router, is reached by overriding it.
	DefaultOpenAIBaseURL = "https://api.openai.com/v1"
)

// openAICompleter calls an OpenAI-compatible Chat Completions API over HTTP. Pointing its
// base URL at a local server like Ollama gives a free, private rewrite with no API key.
type openAICompleter struct {
	// model is the model id sent with each request.
	model string
	// baseURL is the API root the chat completions path is joined onto.
	baseURL string
	// client is the HTTP client requests go through.
	client *http.Client
}

// NewOpenAICompleter returns a Completer that calls an OpenAI-compatible Chat Completions
// API. It reads the API key from OPENAI_API_KEY. An empty model or base URL falls back to
// the OpenAI defaults. A local base URL may run without a key, since servers like Ollama
// ignore the Authorization header.
func NewOpenAICompleter(model, baseURL string) Completer {
	if model == "" {
		model = DefaultOpenAIModel
	}
	if baseURL == "" {
		baseURL = DefaultOpenAIBaseURL
	}
	return &openAICompleter{
		model:   model,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: requestTimeout},
	}
}

// Complete sends one Chat Completions request and returns the reply text. A reply that
// stopped for any reason other than finishing, like running into the token cap, is an
// error rather than silently truncated text.
func (c *openAICompleter) Complete(ctx context.Context, system, user string) (string, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" && c.baseURL == DefaultOpenAIBaseURL {
		return "", fmt.Errorf("OPENAI_API_KEY is not set")
	}

	body, err := json.Marshal(chatRequest{
		Model:     c.model,
		MaxTokens: maxTokens,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions",
		bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	if key != "" {
		req.Header.Set("authorization", "Bearer "+key)
	}

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
		return "", fmt.Errorf("openai api: %s: %s", resp.Status, strings.TrimSpace(string(reply)))
	}

	var out chatResponse
	if err := json.Unmarshal(reply, &out); err != nil {
		return "", fmt.Errorf("openai api: decode reply: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("openai api: reply had no choices")
	}
	choice := out.Choices[0]
	switch choice.FinishReason {
	case "stop", "":
	case "length":
		return "", fmt.Errorf("openai api: reply hit the %d token cap and is truncated", maxTokens)
	case "content_filter":
		return "", fmt.Errorf("openai api: the model declined to rewrite the text")
	default:
		return "", fmt.Errorf("openai api: unexpected finish_reason %q", choice.FinishReason)
	}
	// A stop reply with no text would overwrite the input with nothing under --write, so
	// treat it as an error rather than silent data loss.
	if strings.TrimSpace(choice.Message.Content) == "" {
		return "", fmt.Errorf("openai api: reply had no text content")
	}
	return choice.Message.Content, nil
}

// chatRequest is the POST body for the Chat Completions API.
type chatRequest struct {
	// Model is the model id.
	Model string `json:"model"`
	// MaxTokens caps the reply length.
	MaxTokens int `json:"max_tokens"`
	// Messages is the conversation, a system turn then a user turn.
	Messages []chatMessage `json:"messages"`
}

// chatMessage is one turn in a Chat Completions request.
type chatMessage struct {
	// Role is system, user, or assistant.
	Role string `json:"role"`
	// Content is the turn text.
	Content string `json:"content"`
}

// chatResponse is the part of the Chat Completions reply the rewriter reads.
type chatResponse struct {
	// Choices holds the model's replies. The rewriter reads the first.
	Choices []chatChoice `json:"choices"`
}

// chatChoice is one reply in a Chat Completions response.
type chatChoice struct {
	// Message is the reply turn.
	Message chatMessage `json:"message"`
	// FinishReason says why the model stopped. Anything but stop means the reply is not
	// the whole rewrite.
	FinishReason string `json:"finish_reason"`
}
