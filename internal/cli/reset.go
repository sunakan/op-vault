//go:build darwin

// Package cli implements the CLI subcommand layer
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/sunakan/op-vault/internal/keychain"
	"github.com/sunakan/op-vault/internal/tracing"
)

// ResetCmd implements the reset subcommand
type ResetCmd struct{}

// Run removes the keychain
func (c *ResetCmd) Run(ctx context.Context) error {
	_, span := tracing.Tracer().Start(ctx, "reset")
	defer span.End()

	keychainPath, err := keychain.FilePath()
	if err != nil {
		tracing.SetSpanError(span, err)
		return err
	}

	if _, err := os.Stat(keychainPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "not found: %s\n", keychain.Name())
		return nil
	} else if err != nil {
		tracing.SetSpanError(span, err)
		return err
	}

	if err := keychain.Remove(ctx, keychainPath); err != nil {
		tracing.SetSpanError(span, err)
		return err
	}
	fmt.Fprintf(os.Stderr, "deleted: %s\n", keychain.Name())
	return nil
}
