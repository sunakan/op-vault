//go:build darwin

// Package op provides a 1Password SDK client wrapper.
package op

import (
	"context"
	"fmt"
	"strings"

	onepassword "github.com/1password/onepassword-sdk-go"

	"github.com/sunakan/op-vault/internal/tracing"
)

type sdkClient interface {
	Secrets() onepassword.SecretsAPI
	Vaults() onepassword.VaultsAPI
	Items() onepassword.ItemsAPI
}

// Resolve returns the secret value for the given op:// reference.
func Resolve(ctx context.Context, account, ref string) (string, error) {
	_, span := tracing.Tracer().Start(ctx, "Resolve")
	defer span.End()

	c, err := onepassword.NewClient(ctx,
		onepassword.WithDesktopAppIntegration(account),
		onepassword.WithIntegrationInfo("op-vault", "0.0.0"),
	)
	if err != nil {
		tracing.SetSpanError(span, err)
		return "", err
	}

	value, err := resolveWithClient(ctx, c, ref)
	if err != nil {
		tracing.SetSpanError(span, err)
		return "", err
	}
	return value, nil
}

func resolveWithClient(ctx context.Context, c sdkClient, ref string) (string, error) {
	vaultRef, itemRef, isWebsite := primaryWebsiteRef(ref)
	if !isWebsite {
		value, err := c.Secrets().Resolve(ctx, ref)
		if err != nil {
			msg := err.Error()
			if strings.Contains(msg, "no item matched") || strings.Contains(msg, "no vault matched") {
				return "", fmt.Errorf("not found in 1Password: %s", ref)
			}
			return "", err
		}
		return value, nil
	}

	vaults, err := c.Vaults().List(ctx)
	if err != nil {
		return "", err
	}
	vault, matches := findVault(vaults, vaultRef)
	switch matches {
	case 0:
		return "", fmt.Errorf("not found in 1Password: %s", ref)
	case 1:
		// Continue with the unique vault.
	default:
		return "", fmt.Errorf("more than one vault matched in 1Password: %s", ref)
	}

	items, err := c.Items().List(ctx, vault.ID)
	if err != nil {
		return "", err
	}
	item, matches := findItem(items, itemRef)
	switch matches {
	case 0:
		return "", fmt.Errorf("not found in 1Password: %s", ref)
	case 1:
		// Continue with the unique item.
	default:
		return "", fmt.Errorf("more than one item matched in 1Password: %s", ref)
	}

	if len(item.Websites) == 0 || item.Websites[0].URL == "" {
		return "", fmt.Errorf("not found in 1Password: %s", ref)
	}
	return item.Websites[0].URL, nil
}

func primaryWebsiteRef(ref string) (vaultRef, itemRef string, ok bool) {
	if !strings.HasPrefix(ref, "op://") {
		return "", "", false
	}
	parts := strings.Split(ref[len("op://"):], "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || !strings.EqualFold(parts[2], "website") {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func findVault(vaults []onepassword.VaultOverview, ref string) (match onepassword.VaultOverview, matches int) {
	for _, vault := range vaults {
		if vault.ID == ref {
			return vault, 1
		}
	}

	for _, vault := range vaults {
		if strings.EqualFold(vault.Title, ref) {
			match = vault
			matches++
		}
	}
	return match, matches
}

func findItem(items []onepassword.ItemOverview, ref string) (match onepassword.ItemOverview, matches int) {
	for _, item := range items {
		if item.ID == ref {
			return item, 1
		}
	}

	for _, item := range items {
		if strings.EqualFold(item.Title, ref) {
			match = item
			matches++
		}
	}
	return match, matches
}
