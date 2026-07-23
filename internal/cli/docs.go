//go:build darwin

package cli

import (
	"context"
	"fmt"

	opvault "github.com/sunakan/op-vault"
	"github.com/sunakan/op-vault/internal/tracing"
)

// DocsCmd implements the docs subcommand
type DocsCmd struct{}

// Run prints the embedded README to stdout
func (c *DocsCmd) Run(ctx context.Context) error {
	_, span := tracing.Tracer().Start(ctx, "docs")
	defer span.End()
	fmt.Print(opvault.Readme)
	return nil
}
