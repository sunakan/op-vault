//go:build darwin

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sunakan/op-keychain/internal/keychain"
	"github.com/sunakan/op-keychain/internal/op"
)

type RefreshCmd struct {
	Account    string            `short:"a" name:"account" optional:"" env:"OP_ACCOUNT" help:"1Password account name"`
	KC         keychain.Keychain `kong:"-"`
	OP         op.Client         `kong:"-"`
	AppVersion string            `kong:"-"`
}

func (c *RefreshCmd) Run() error {
	exists, err := c.KC.Exists()
	if err != nil {
		return err
	}
	if !exists {
		fmt.Println("no keychain")
		return nil
	}

	if err := c.KC.Unlock(); err != nil {
		return err
	}

	services, err := c.KC.ListServices()
	if err != nil {
		return err
	}
	if len(services) == 0 {
		fmt.Println("no cache")
		return nil
	}

	// 各 service から entry を収集
	type target struct {
		svc   string
		entry keychain.Entry
	}
	var targets []target
	for _, svc := range services {
		raw, err := c.KC.Get(svc)
		if err != nil {
			continue
		}
		var e keychain.Entry
		if err := json.Unmarshal([]byte(raw), &e); err != nil {
			continue
		}
		targets = append(targets, target{svc: svc, entry: e})
	}
	if len(targets) == 0 {
		fmt.Println("no cache")
		return nil
	}

	opClient, err := c.opClient()
	if err != nil {
		return fmt.Errorf("failed to initialize 1Password client: %w", err)
	}

	ctx := context.Background()
	updated, failed := 0, 0

	for _, t := range targets {
		account := t.entry.Account
		if c.Account != "" {
			account = c.Account
		}

		value, err := opClient.Resolve(ctx, t.entry.Ref)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skip (failed): %s\n", t.entry.Ref)
			failed++
			continue
		}

		name, err := opClient.ItemTitle(ctx, t.entry.Ref)
		if err != nil {
			name = t.entry.Name
		}

		e := keychain.Entry{
			Ref:     t.entry.Ref,
			Name:    name,
			Value:   value,
			Account: account,
		}
		data, err := json.Marshal(e)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skip (failed): %s\n", t.entry.Ref)
			failed++
			continue
		}

		if err := c.KC.Set(t.svc, string(data)); err != nil {
			fmt.Fprintf(os.Stderr, "  skip (failed): %s\n", t.entry.Ref)
			failed++
			continue
		}

		fmt.Printf("  refreshed: %s\n", t.entry.Ref)
		updated++
	}

	fmt.Printf("done: %d updated, %d failed\n", updated, failed)
	return nil
}

func (c *RefreshCmd) opClient() (op.Client, error) {
	if c.OP != nil {
		return c.OP, nil
	}
	return op.NewClient(context.Background(), c.AppVersion)
}
