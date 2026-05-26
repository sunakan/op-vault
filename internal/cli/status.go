//go:build darwin

package cli

import (
	"context"
	"fmt"

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
		fmt.Println("status: not initialized")
		return nil
	}

	if !result.Unlocked {
		fmt.Println("status: locked")
		fmt.Printf("path: %s\n", result.Path)
		return nil
	}

	noun := "entries"
	if result.EntryCount == 1 {
		noun = "entry"
	}
	fmt.Println("status: unlocked")
	fmt.Printf("path: %s\n", result.Path)
	fmt.Printf("cache: %d %s\n", result.EntryCount, noun)
	return nil
}
