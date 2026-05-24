//go:build darwin

// Package op provides a 1Password SDK client wrapper.
package op

import (
	"context"
	"fmt"
	"strings"

	onepassword "github.com/1password/onepassword-sdk-go"

	"github.com/sunakan/op-keychain/internal/tracing"
)

// Resolve returns the secret value for the given op:// reference.
func Resolve(ctx context.Context, account, ref string) (string, error) {
	_, span := tracing.Tracer().Start(ctx, "Resolve")
	defer span.End()

	c, err := onepassword.NewClient(ctx,
		onepassword.WithDesktopAppIntegration(account),
		onepassword.WithIntegrationInfo("op-keychain", "0.0.0"),
	)
	if err != nil {
		tracing.SetSpanError(span, err)
		return "", err
	}

	value, err := c.Secrets().Resolve(ctx, ref)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "no item matched") || strings.Contains(msg, "no vault matched") {
			err = fmt.Errorf("not found in 1Password: %s", ref)
		}
		tracing.SetSpanError(span, err)
		return "", err
	}
	return value, nil
}
