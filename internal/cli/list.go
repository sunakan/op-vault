//go:build darwin

// Package cli implements the CLI subcommand layer
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/sunakan/op-vault/internal/keychain"
	"github.com/sunakan/op-vault/internal/tracing"
)

// ListCmd implements the list subcommand
type ListCmd struct{}

// Run prints all cached op:// refs and their update times to stdout.
func (c *ListCmd) Run(ctx context.Context) error {
	_, span := tracing.Tracer().Start(ctx, "list")
	defer span.End()

	entries, err := keychain.List(ctx)
	if err != nil {
		tracing.SetSpanError(span, err)
		var notFound *keychain.NotFoundError
		if errors.As(err, &notFound) {
			return errors.New("keychain not found: run 'op-vault init'")
		}
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "NAME\tUPDATED AT\n")
	for _, e := range entries {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", e.Ref, e.UpdatedAt)
	}
	return w.Flush()
}
