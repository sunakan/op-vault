//go:build darwin

// Package cli implements the CLI subcommand layer
package cli

import (
	"context"
	"fmt"

	"github.com/sunakan/op-vault/internal/tracing"
)

// VersionCmd implements the version subcommand
type VersionCmd struct {
	Version string `kong:"-"`
}

// Run prints the version string to stdout
func (c *VersionCmd) Run(ctx context.Context) error {
	_, span := tracing.Tracer().Start(ctx, "version")
	defer span.End()
	fmt.Println(c.Version)
	return nil
}
