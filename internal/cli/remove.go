//go:build darwin

package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/sunakan/op-keychain/internal/keychain"
	"github.com/sunakan/op-keychain/internal/op"
)

type RemoveCmd struct {
	Ref string            `arg:"" help:"op://vault/item[/field]"`
	KC  keychain.Keychain `kong:"-"`
}

func (c *RemoveCmd) Run() error {
	if _, err := op.ParseRef(c.Ref); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}

	exists, err := c.KC.Exists()
	if err != nil {
		return err
	}
	if !exists {
		fmt.Fprintln(os.Stderr, "error: no keychain")
		os.Exit(1)
	}

	svc := keychain.Service(c.Ref)
	err = c.KC.Remove(svc)
	if errors.Is(err, keychain.ErrLocked) {
		if unlockErr := c.KC.Unlock(); unlockErr != nil {
			return unlockErr
		}
		err = c.KC.Remove(svc)
	}
	if errors.Is(err, keychain.ErrNotFound) {
		fmt.Fprintf(os.Stderr, "error: cache not found: %s\n", c.Ref)
		os.Exit(1)
	}
	if err != nil {
		return err
	}

	fmt.Printf("removed: %s\n", c.Ref)
	return nil
}
