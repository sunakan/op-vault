//go:build darwin

package keychain

import (
	"context"
	"os"

	"github.com/sunakan/op-vault/internal/tracing"
)

// StatusResult holds the state of the keychain.
type StatusResult struct {
	Initialized  bool
	Unlocked     bool
	EntryCount   int
	Path         string
	LockInterval uint32
	LockOnSleep  bool
}

// Status returns the current state of the keychain.
func Status(ctx context.Context) (StatusResult, error) {
	_, span := tracing.Tracer().Start(ctx, "Status")
	defer span.End()

	path, err := FilePath()
	if err != nil {
		tracing.SetSpanError(span, err)
		return StatusResult{}, err
	}

	if _, err := os.Stat(path); err != nil {
		return StatusResult{Initialized: false, Path: path}, nil
	}

	unlocked, err := cgoGetStatus(path)
	if err != nil {
		tracing.SetSpanError(span, err)
		return StatusResult{}, err
	}

	if !unlocked {
		return StatusResult{Initialized: true, Unlocked: false, Path: path}, nil
	}

	count, err := cgoCountItems(path)
	if err != nil {
		tracing.SetSpanError(span, err)
		return StatusResult{}, err
	}

	settings, err := cgoGetSettings(path)
	if err != nil {
		tracing.SetSpanError(span, err)
		return StatusResult{}, err
	}

	return StatusResult{
		Initialized:  true,
		Unlocked:     true,
		EntryCount:   count,
		Path:         path,
		LockInterval: settings.LockInterval,
		LockOnSleep:  settings.LockOnSleep,
	}, nil
}
