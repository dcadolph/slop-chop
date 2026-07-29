package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dcadolph/slop-chop/cmd/config"
	"github.com/dcadolph/slop-chop/internal/mcp"
	"github.com/dcadolph/slop-chop/internal/rewrite"
	"github.com/dcadolph/slop-chop/internal/sanitize"
)

// mcpCmd builds the mcp subcommand, which serves slop-chop over the Model Context Protocol.
func mcpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run as a Model Context Protocol server over stdio.",
		Long: `mcp speaks the Model Context Protocol on stdin and stdout, so an MCP client such as
Claude Desktop, Claude Code, or Cursor can chop text on demand. It serves three tools: chop
cleans text, check reports the tells without changing anything, and presets lists the built-in
packs. The rules pass runs here on your machine and uploads nothing. The model rewrite stays
off unless a call asks for it. The flags set the server's defaults, which a call can override
for itself, and the same profile, presets, and voice as the other commands apply.`,
		Args: cobra.NoArgs,
		RunE: runMCP,
	}
	f := cmd.Flags()
	f.AddFlag(&config.FlagProfile)
	f.AddFlag(&config.FlagDialect)
	f.AddFlag(&config.FlagPreset)
	f.AddFlag(&config.FlagVoice)
	f.AddFlag(&config.FlagProvider)
	f.AddFlag(&config.FlagModel)
	f.AddFlag(&config.FlagBaseURL)
	return cmd
}

// runMCP serves the tools on stdio until the client disconnects.
func runMCP(cmd *cobra.Command, _ []string) error {
	// A base URL is only wired up for the OpenAI provider, so pointing one at the default
	// backend would silently bill the paid API instead of the local model the caller meant.
	if config.BaseURL() != "" && config.Provider() != string(rewrite.ProviderOpenAI) {
		return fmt.Errorf("--base-url only applies to --provider openai")
	}
	// Build a sanitizer once up front so a bad profile, preset, or dialect fails the launch
	// with a clear message, rather than surfacing on the first tool call.
	if _, _, err := newSanitizer(); err != nil {
		return err
	}
	srv := mcp.NewServer(mcp.SanitizerFunc(mcpSanitizer), mcpRewriter(), resolveVersion())
	return srv.Run(cmd.Context())
}

// mcpSanitizer builds the rules engine for one MCP call. An override the call left empty
// falls back to the flag the server started with, so a call naming nothing gets exactly the
// configuration the server was launched with.
func mcpSanitizer(presets []string, dialect string) (*sanitize.Sanitizer, sanitize.Profile, error) {
	if len(presets) == 0 {
		presets = splitList(config.Preset())
	}
	if dialect == "" {
		dialect = config.Dialect()
	}
	return sanitizerFor(presets, dialect)
}

// mcpRewriter returns the model rewrite backend for the MCP server. The completer is built
// per call rather than at launch, so a server started without a key still serves the free
// deterministic tools and reports the missing key only to the call that asked for the model.
func mcpRewriter() mcp.Rewriter {
	return mcp.RewriterFunc(func(ctx context.Context, text string, tone []string) (string, error) {
		completer, err := newRewriteCompleter()
		if err != nil {
			return "", err
		}
		return rewritePass(ctx, completer, tone, text)
	})
}
