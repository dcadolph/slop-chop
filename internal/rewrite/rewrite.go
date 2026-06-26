// Package rewrite is the optional Layer 2 pass. It hands text to a model to do what
// the deterministic rules cannot, like reworking a sentence to drop a semicolon or
// bending the text toward a chosen voice.
package rewrite

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
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

// Rewrite returns the text rewritten to read like a person wrote it.
func (r *Rewriter) Rewrite(ctx context.Context, text string) (string, error) {
	out, err := r.completer.Complete(ctx, buildSystem(r.tone), text)
	if err != nil {
		return "", fmt.Errorf("rewrite: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// buildSystem assembles the instruction that tells the model how to clean the text.
func buildSystem(tone []string) string {
	var b strings.Builder
	b.WriteString("You rewrite text so it reads like a person wrote it, not a chatbot. ")
	b.WriteString("Keep the meaning and the facts unchanged. Do not add or remove ideas.\n\n")
	b.WriteString("Remove the tells of AI writing:\n")
	b.WriteString("- No em-dashes. Recast the sentence or use a comma or a period.\n")
	b.WriteString("- No semicolons joining clauses. Split them into separate sentences.\n")
	b.WriteString("- Drop filler openers like \"In summary\" and \"To be honest\".\n")
	b.WriteString("- Cut buzzwords like \"comprehensive\", \"robust\", and \"leverage\".\n")
	b.WriteString("- Vary sentence length. Avoid the flat, even cadence models fall into.\n")
	b.WriteString("- Use plain words and contractions where they fit.\n\n")
	if len(tone) > 0 {
		b.WriteString("Match this voice:\n")
		for _, t := range tone {
			b.WriteString("- ")
			b.WriteString(t)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("Return only the rewritten text. No preamble, no quotes, no notes.")
	return b.String()
}

// NewAnthropicCompleter returns a Completer backed by the Anthropic API. The client
// reads the API key from the ANTHROPIC_API_KEY environment variable.
func NewAnthropicCompleter(model string) Completer {
	if model == "" {
		model = DefaultModel
	}
	client := anthropic.NewClient()
	return CompleterFunc(func(ctx context.Context, system, user string) (string, error) {
		resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(model),
			MaxTokens: 16000,
			System:    []anthropic.TextBlockParam{{Text: system}},
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
			},
		})
		if err != nil {
			return "", err
		}
		var b strings.Builder
		for _, block := range resp.Content {
			if t, ok := block.AsAny().(anthropic.TextBlock); ok {
				b.WriteString(t.Text)
			}
		}
		return b.String(), nil
	})
}
