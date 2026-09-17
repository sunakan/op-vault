//go:build darwin

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/sunakan/op-vault/internal/keychain"
	"github.com/sunakan/op-vault/internal/tracing"
)

// ClearCmd implements the clear subcommand.
type ClearCmd struct{}

// Run removes all cache entries from the keychain without deleting it.
func (c *ClearCmd) Run(ctx context.Context) error {
	_, span := tracing.Tracer().Start(ctx, "clear")
	defer span.End()

	keychainPath, err := keychain.FilePath()
	if err != nil {
		tracing.SetSpanError(span, err)
		return err
	}

	if err := keychain.Clear(ctx, keychainPath); err != nil {
		tracing.SetSpanError(span, err)
		if _, ok := errors.AsType[*keychain.NotFoundError](err); ok {
			return errors.New("keychain not found: run 'op-vault init'")
		}
		return err
	}
	fmt.Fprintf(os.Stderr, "cleared: %s\n", keychain.Name())
	return nil
}
