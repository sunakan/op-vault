//go:build darwin

// Package cli implements the CLI subcommand layer
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sunakan/op-vault/internal/keychain"
	"github.com/sunakan/op-vault/internal/op"
	"github.com/sunakan/op-vault/internal/tracing"
)

// ReadCmd implements the read subcommand
type ReadCmd struct {
	Ref     string `arg:"" help:"op://VaultName/ItemName(or UUID)/password"`
	Account string `short:"a" env:"OP_ACCOUNT" optional:"" help:"1Password account name or UUID"`
}

// Run reads a secret from cache or 1Password.
func (c *ReadCmd) Run(ctx context.Context) error {
	_, span := tracing.Tracer().Start(ctx, "read")
	defer span.End()

	if !isValidRef(c.Ref) {
		err := errors.New("invalid ref format: must be op://<vault>/<item>/[section/]field (e.g. op://my-vault/my-item/password)")
		tracing.SetSpanError(span, err)
		return err
	}

	if c.Account == "" {
		err := errors.New("account is required: set OP_ACCOUNT or use --account (-a)")
		tracing.SetSpanError(span, err)
		return err
	}

	value, err := keychain.Get(ctx, c.Account, c.Ref)
	// early return on cache hit to keep error-handling branches at the top level
	if err == nil {
		fmt.Print(value)
		return nil
	}

	var cacheMiss *keychain.CacheMissError
	if errors.As(err, &cacheMiss) {
		value, err = op.Resolve(ctx, c.Account, c.Ref)
		if err != nil {
			tracing.SetSpanError(span, err)
			return err
		}
		if setErr := keychain.Set(ctx, c.Account, c.Ref, value); setErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to cache secret: %v\n", setErr)
		}
		fmt.Print(value)
		return nil
	}

	var notFound *keychain.NotFoundError
	if errors.As(err, &notFound) {
		tracing.SetSpanError(span, err)
		return errors.New("keychain not found: run 'op-vault init'")
	}

	tracing.SetSpanError(span, err)
	return err
}

// isValidRef checks that ref follows op://<vault>/<item>/field format.
// strings.Count checks for at least 2 slashes after op://, covering vault/item/field.
func isValidRef(ref string) bool {
	if !strings.HasPrefix(ref, "op://") {
		return false
	}
	return strings.Count(ref[len("op://"):], "/") >= 2
}
