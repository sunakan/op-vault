//go:build darwin

package keychain

import (
	"context"
	"os"

	"github.com/sunakan/op-vault/internal/tracing"
)

// ListEntry holds a cached ref and its last update time.
type ListEntry struct {
	Ref       string
	UpdatedAt string // "YYYY-MM-DD HH:MM:SS" local time, or "" if unavailable
}

// List returns all cached entries in the keychain.
// Returns NotFoundError if the keychain does not exist.
func List(ctx context.Context) ([]ListEntry, error) {
	_, span := tracing.Tracer().Start(ctx, "List")
	defer span.End()

	path, err := FilePath()
	if err != nil {
		tracing.SetSpanError(span, err)
		return nil, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		e := &NotFoundError{Path: path}
		tracing.SetSpanError(span, e)
		return nil, e
	}

	entries, err := cgoList(path)
	if err != nil {
		tracing.SetSpanError(span, err)
		return nil, err
	}
	return entries, nil
}
