//go:build darwin

package keychain

import (
	"context"
	"os"

	"github.com/sunakan/op-vault/internal/tracing"
)

// DeleteItem removes a single cached entry identified by account and ref.
func DeleteItem(ctx context.Context, account, ref string) error {
	_, span := tracing.Tracer().Start(ctx, "DeleteItem")
	defer span.End()

	path, err := FilePath()
	if err != nil {
		tracing.SetSpanError(span, err)
		return err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		e := &NotFoundError{Path: path}
		tracing.SetSpanError(span, e)
		return e
	}

	if err := cgoDeleteItem(path, ref, account); err != nil {
		tracing.SetSpanError(span, err)
		return err
	}
	return nil
}
