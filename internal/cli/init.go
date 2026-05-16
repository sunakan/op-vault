//go:build darwin

// Package cli implements the CLI subcommand layer
package cli

import (
	"context"
	"errors"

	"github.com/sunakan/op-keychain/internal/tracing"
)

// InitCmd implements the init subcommand
type InitCmd struct {
}

// Run creates and initializes the keychain
func (c *InitCmd) Run(ctx context.Context) error {
	_, span := tracing.Tracer().Start(ctx, "init")
	defer span.End()
	return errors.New("not implemented")
}
