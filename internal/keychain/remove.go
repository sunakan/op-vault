//go:build darwin

package keychain

import (
	"context"

	"github.com/sunakan/op-keychain/internal/tracing"
)

// Remove removes a keychain at the given path.
func Remove(ctx context.Context, path string) error {
	_, span := tracing.Tracer().Start(ctx, "Remove")
	defer span.End()
	if err := cgoDelete(path); err != nil {
		tracing.SetSpanError(span, err)
		return err
	}
	return nil
}
