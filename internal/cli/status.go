//go:build darwin

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/sunakan/op-keychain/internal/keychain"
	"github.com/sunakan/op-keychain/internal/tracing"
)

// StatusCmd implements the status subcommand.
type StatusCmd struct{}

// Run prints the current state of the keychain to stdout.
func (c *StatusCmd) Run(ctx context.Context) error {
	_, span := tracing.Tracer().Start(ctx, "status")
	defer span.End()

	result, err := keychain.Status(ctx)
	if err != nil {
		tracing.SetSpanError(span, err)
		return err
	}

	if !result.Initialized {
		fmt.Fprintln(os.Stdout, "status: not initialized")
		return nil
	}

	if !result.Unlocked {
		fmt.Fprintln(os.Stdout, "status: locked")
		fmt.Fprintf(os.Stdout, "path: %s\n", result.Path)
		return nil
	}

	noun := "entries"
	if result.EntryCount == 1 {
		noun = "entry"
	}
	fmt.Fprintln(os.Stdout, "status: unlocked")
	fmt.Fprintf(os.Stdout, "path: %s\n", result.Path)
	fmt.Fprintf(os.Stdout, "cache: %d %s\n", result.EntryCount, noun)
	return nil
}
