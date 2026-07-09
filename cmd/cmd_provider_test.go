package cmd

import (
	"strings"
	"testing"
)

// TestFixProviderRouting checks that newRewriteCompleter builds the backend the flags name.
// With both keys cleared, each provider fails on its own missing-key message, which proves
// the routing without making a network call.
func TestFixProviderRouting(t *testing.T) {
	// t.Setenv forbids t.Parallel, so this test runs serially with both keys cleared.
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	tests := []struct {
		Name string
		Args []string
		Want string
	}{
		{Name: "default is anthropic", Args: []string{"fix", "--rewrite"}, Want: "ANTHROPIC_API_KEY is not set"},
		{Name: "provider openai", Args: []string{"fix", "--rewrite", "--provider", "openai"}, Want: "OPENAI_API_KEY is not set"},
		{Name: "unknown provider", Args: []string{"fix", "--rewrite", "--provider", "grok"}, Want: "unknown provider"},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			_, _, err := runCLI(t, test.Args, "some slop to rewrite")
			if err == nil || !strings.Contains(err.Error(), test.Want) {
				t.Errorf("err = %v, want containing %q", err, test.Want)
			}
		})
	}
}

// TestFixProviderGuards checks the flag combinations rejected before any model call.
func TestFixProviderGuards(t *testing.T) {
	tests := []struct {
		Name string
		Args []string
		Want string
	}{
		{
			Name: "provider needs rewrite",
			Args: []string{"fix", "--provider", "openai"},
			Want: "--provider needs --rewrite",
		},
		{
			Name: "base-url needs rewrite",
			Args: []string{"fix", "--base-url", "http://localhost:11434/v1"},
			Want: "--base-url needs --rewrite",
		},
		{
			Name: "base-url needs openai",
			Args: []string{"fix", "--rewrite", "--base-url", "http://localhost:11434/v1", "--provider", "anthropic"},
			Want: "--base-url only applies to --provider openai",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			_, _, err := runCLI(t, test.Args, "text")
			if err == nil || !strings.Contains(err.Error(), test.Want) {
				t.Errorf("err = %v, want containing %q", err, test.Want)
			}
		})
	}
}
