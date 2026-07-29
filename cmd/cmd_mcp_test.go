package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dcadolph/slop-chop/cmd/config"
)

// TestMCPSanitizer checks how one MCP call's preset and dialect meet the flags the server was
// started with: an override the call names replaces the server's, and an override it leaves
// out falls back to the server's own setting.
func TestMCPSanitizer(t *testing.T) {
	tests := []struct {
		ServerPreset  string
		ServerDialect string
		Presets       []string
		Dialect       string
		In            string
		WantOut       string
		WantErrSub    string
	}{{ // Test 0: naming nothing falls back to the preset the server started with.
		ServerPreset: "plain", In: "we utilize it", WantOut: "we use it",
	}, { // Test 1: a call's preset applies when the server started with none.
		Presets: []string{"plain"}, In: "we utilize it", WantOut: "we use it",
	}, { // Test 2: a call's preset replaces the server's rather than stacking on it, so a
		// swap only the server's pack carries no longer fires.
		ServerPreset: "academic", Presets: []string{"plain"},
		In: "in light of the fact that it works", WantOut: "in light of the fact that it works",
	}, { // Test 3: naming nothing falls back to the dialect the server started with.
		ServerDialect: "american", In: "the colour of it", WantOut: "the color of it",
	}, { // Test 4: a call's dialect replaces the server's.
		ServerDialect: "american", Dialect: "british",
		In: "the color of it", WantOut: "the colour of it",
	}, { // Test 5: an unknown preset from a call names the packs that exist.
		Presets: []string{"bogus"}, In: "we ship it", WantErrSub: "unknown preset",
	}, { // Test 6: an unknown dialect from a call is refused.
		Dialect: "klingon", In: "we ship it", WantErrSub: "unknown dialect",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			config.Reset()
			t.Cleanup(config.Reset)
			t.Setenv("SLOP_CHOP_PRESET", test.ServerPreset)
			t.Setenv("SLOP_CHOP_DIALECT", test.ServerDialect)

			san, _, err := mcpSanitizer(test.Presets, test.Dialect)
			if test.WantErrSub != "" {
				if err == nil || !strings.Contains(err.Error(), test.WantErrSub) {
					t.Fatalf("err = %v, want it to contain %q", err, test.WantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if got, _ := san.Fix(test.In); got != test.WantOut {
				t.Errorf("out = %q, want %q", got, test.WantOut)
			}
		})
	}
}

// TestMCPCommandRegistered checks that the root command carries the mcp subcommand and that
// it takes no positional arguments, since the protocol runs over stdio.
func TestMCPCommandRegistered(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
	var found bool
	for _, sub := range rootCmd().Commands() {
		if sub.Name() == "mcp" {
			found = true
		}
	}
	if !found {
		t.Fatal("the root command has no mcp subcommand")
	}
	if _, _, err := runCLI(t, []string{"mcp", "somefile.md"}, ""); err == nil {
		t.Error("mcp accepted a positional argument, want it refused")
	}
}

// TestMCPBaseURLNeedsOpenAI checks that a base URL aimed at the default backend is refused at
// launch. Only the OpenAI provider wires one up, so letting it through would send the text to
// the paid API rather than the local model the caller pointed at.
func TestMCPBaseURLNeedsOpenAI(t *testing.T) {
	t.Setenv("SLOP_CHOP_BASE_URL", "http://localhost:11434/v1")
	_, _, err := runCLI(t, []string{"mcp"}, "")
	if err == nil || !strings.Contains(err.Error(), "--base-url only applies to --provider openai") {
		t.Errorf("err = %v, want a base-url provider mismatch error", err)
	}
}
