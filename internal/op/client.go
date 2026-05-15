//go:build darwin

package op

import (
	"context"
	"fmt"
	"os"

	onepassword "github.com/1password/onepassword-sdk-go"
)

type Client interface {
	Resolve(ctx context.Context, ref string) (string, error)
	ItemTitle(ctx context.Context, ref string) (string, error)
}

type sdkClient struct {
	op *onepassword.Client
}

func NewClient(ctx context.Context, integrationVersion string) (Client, error) {
	account := os.Getenv("OP_ACCOUNT")
	c, err := onepassword.NewClient(ctx,
		onepassword.WithDesktopAppIntegration(account),
		onepassword.WithIntegrationInfo("op-keychain", integrationVersion),
	)
	if err != nil {
		return nil, err
	}
	return &sdkClient{op: c}, nil
}

func (c *sdkClient) Resolve(ctx context.Context, ref string) (string, error) {
	return c.op.Secrets().Resolve(ctx, ref)
}

// ItemTitle は vault/item 名でルックアップして item title を返す。失敗時は空文字を返す。
// ref からパースした名前で一致を探す（ID ではなく名前ベース）。
func (c *sdkClient) ItemTitle(ctx context.Context, ref string) (string, error) {
	r, err := ParseRef(ref)
	if err != nil {
		return "", err
	}
	vaults, err := c.op.Vaults().List(ctx)
	if err != nil {
		return "", err
	}
	var vaultID string
	for _, v := range vaults {
		if v.Title == r.Vault {
			vaultID = v.ID
			break
		}
	}
	if vaultID == "" {
		return "", fmt.Errorf("vault not found: %s", r.Vault)
	}
	items, err := c.op.Items().List(ctx, vaultID)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item.Title == r.Item {
			return item.Title, nil
		}
	}
	return "", fmt.Errorf("item not found: %s", r.Item)
}
