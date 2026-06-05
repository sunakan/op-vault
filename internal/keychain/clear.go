//go:build darwin

package keychain

import (
	"context"
	"os"

	"github.com/sunakan/op-vault/internal/tracing"
)

// Clear removes all cache entries from the keychain at path without deleting the keychain itself.
func Clear(ctx context.Context, path string) error {
	_, span := tracing.Tracer().Start(ctx, "Clear")
	defer span.End()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &NotFoundError{Path: path}
	}
	if err := cgoClearItems(path); err != nil {
		tracing.SetSpanError(span, err)
		return err
	}
	return nil
}
