//go:build darwin

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/sunakan/op-keychain/internal/keychain"
	"github.com/sunakan/op-keychain/internal/tracing"
)

// SetCmd implements the set subcommand.
type SetCmd struct {
	Ref     string `arg:"" help:"op://VaultName/ItemName/field"`
	Value   string `arg:"" help:"Secret value to cache"`
	Account string `short:"a" env:"OP_ACCOUNT" optional:"" help:"1Password account name or UUID"`
}

// Run caches a secret in the keychain.
func (c *SetCmd) Run(ctx context.Context) error {
	_, span := tracing.Tracer().Start(ctx, "set")
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

	if err := keychain.Set(ctx, c.Account, c.Ref, c.Value); err != nil {
		tracing.SetSpanError(span, err)
		var notFound *keychain.NotFoundError
		if errors.As(err, &notFound) {
			return errors.New("keychain not found: run 'op-keychain init'")
		}
		return err
	}

	fmt.Fprintf(os.Stderr, "cached: %s\n", c.Ref)
	return nil
}
