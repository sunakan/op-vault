//go:build darwin

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/sunakan/op-keychain/internal/keychain"
	"github.com/sunakan/op-keychain/internal/op"
)

type ReadCmd struct {
	Ref        string            `arg:"" help:"op://vault/item[/field]"`
	Account    string            `short:"a" name:"account" optional:"" env:"OP_ACCOUNT" help:"1Password account name"`
	KC         keychain.Keychain `kong:"-"`
	OP         op.Client         `kong:"-"` // nil の場合は Run() 内で op.NewClient を呼ぶ
	AppVersion string            `kong:"-"` // op.NewClient に渡す integration version
}

func (c *ReadCmd) Run() error {
	if _, err := op.ParseRef(c.Ref); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		osExit(2)
	}

	exists, err := c.KC.Exists()
	if err != nil {
		return err
	}
	if !exists {
		if err := AutoCreate(c.KC); err != nil {
			return err
		}
	}

	// cache hit 試行（unlock なし）
	svc := keychain.Service(c.Ref)
	if raw, getErr := c.KC.Get(svc); getErr == nil {
		var e keychain.Entry
		if jsonErr := json.Unmarshal([]byte(raw), &e); jsonErr == nil {
			slog.Debug("cache hit", "ref", c.Ref)
			fmt.Print(e.Value)
			return nil
		}
		slog.Debug("cache hit but JSON parse failed, treating as miss", "ref", c.Ref)
	}

	// 1Password SDK で resolve
	opClient, err := c.opClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize 1Password client: %v\n", err)
		osExit(1)
	}

	ctx := context.Background()
	value, err := opClient.Resolve(ctx, c.Ref)
	if err != nil {
		slog.Debug("resolve failed", "ref", c.Ref, "err", err)
		fmt.Fprintf(os.Stderr, "error: failed to resolve: %s\n", c.Ref)
		osExit(1)
	}

	name, err := opClient.ItemTitle(ctx, c.Ref)
	if err != nil {
		slog.Debug("failed to get item title", "ref", c.Ref, "err", err)
		name = ""
	}

	entry := keychain.Entry{
		Ref:     c.Ref,
		Name:    name,
		Value:   value,
		Account: c.Account,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	// unlock なしで保存を試み、ロック時のみ unlock してリトライ
	if setErr := c.KC.Set(svc, string(data)); setErr != nil {
		if errors.Is(setErr, keychain.ErrLocked) {
			if unlockErr := c.KC.Unlock(); unlockErr != nil {
				return unlockErr
			}
			if setErr = c.KC.Set(svc, string(data)); setErr != nil {
				return setErr
			}
		} else {
			return setErr
		}
	}

	fmt.Print(value)
	return nil
}

func (c *ReadCmd) opClient() (op.Client, error) {
	if c.OP != nil {
		return c.OP, nil
	}
	return op.NewClient(context.Background(), c.AppVersion)
}
