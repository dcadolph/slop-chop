package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
)

// Execute runs the root command and exits the process with the matching code: 0 clean,
// 1 findings in check mode, 2 on error.
func Execute() {
	// A first interrupt cancels the context so a long rewrite call unwinds cleanly. A
	// second one falls back to the default hard stop.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	switch err := rootCmd().ExecuteContext(ctx); {
	case err == nil:
	case errors.Is(err, errFindings):
		os.Exit(codeFindings)
	default:
		fmt.Fprintln(os.Stderr, "slop-chop:", err)
		os.Exit(codeError)
	}
}
