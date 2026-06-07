//go:build darwin

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/sunakan/op-vault/internal/keychain"
	"github.com/sunakan/op-vault/internal/op"
	"github.com/sunakan/op-vault/internal/tracing"
)

// RefreshCmd implements the refresh subcommand
type RefreshCmd struct {
	Prune bool `short:"p" help:"Remove entries not found in 1Password from the keychain"`
}

// Run re-fetches all cached secrets from 1Password and prints results to stdout.
// OP_ACCOUNT is not required — account is read from each keychain entry.
func (c *RefreshCmd) Run(ctx context.Context) error {
	_, span := tracing.Tracer().Start(ctx, "refresh")
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

	type row struct {
		ref       string
		status    string
		updatedAt string
	}
	rows := make([]row, 0, len(entries))

	for _, e := range entries {
		value, resolveErr := op.Resolve(ctx, e.Account, e.Ref)
		if resolveErr != nil {
			status := "error"
			if strings.Contains(resolveErr.Error(), "not found in 1Password") {
				if c.Prune {
					if delErr := keychain.DeleteItem(ctx, e.Account, e.Ref); delErr != nil {
						status = "error"
					} else {
						status = "removed"
					}
				} else {
					status = "not found"
				}
			}
			rows = append(rows, row{ref: e.Ref, status: status, updatedAt: "-"})
			continue
		}
		if setErr := keychain.Set(ctx, e.Account, e.Ref, value); setErr != nil {
			rows = append(rows, row{ref: e.Ref, status: "error", updatedAt: "-"})
			continue
		}
		rows = append(rows, row{
			ref:       e.Ref,
			status:    "refreshed",
			updatedAt: time.Now().Format("2006-01-02 15:04:05"),
		})
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "NAME\tSTATUS\tUPDATED AT\n")
	for _, r := range rows {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", r.ref, r.status, r.updatedAt)
	}
	return w.Flush()
}
