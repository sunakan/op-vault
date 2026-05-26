//go:build darwin

// Package cli implements the CLI subcommand layer
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/sunakan/op-keychain/internal/keychain"
	"github.com/sunakan/op-keychain/internal/tracing"
)

// InitCmd implements the init subcommand
type InitCmd struct{}

// Run creates and initializes the keychain
func (c *InitCmd) Run(ctx context.Context) error {
	_, span := tracing.Tracer().Start(ctx, "init")
	defer span.End()

	keychainPath, err := keychain.FilePath()
	if err != nil {
		tracing.SetSpanError(span, err)
		return err
	}

	if _, err := os.Stat(keychainPath); err == nil {
		fmt.Fprintf(os.Stderr, "keychain already exists: %s\n", keychain.Name())
		return nil
	}

	password, err := keychain.ReadPassword(ctx)
	if err != nil {
		tracing.SetSpanError(span, err)
		return err
	}

	if err := keychain.Create(ctx, keychainPath, password); err != nil {
		tracing.SetSpanError(span, err)
		return err
	}
	fmt.Fprintf(os.Stderr, "Initialized %s\n", keychainPath)
	return nil
}
