//go:build darwin

package cli

import (
	"fmt"
	"os"

	"github.com/sunakan/op-keychain/internal/keychain"
)

type SetIdleTimeoutCmd struct {
	Seconds int               `arg:"" help:"Timeout in seconds (positive integer)"`
	KC      keychain.Keychain `kong:"-"`
}

func (c *SetIdleTimeoutCmd) Run() error {
	if c.Seconds <= 0 {
		fmt.Fprintf(os.Stderr, "error: seconds must be a positive integer: %d\n", c.Seconds)
		os.Exit(2)
	}

	exists, err := c.KC.Exists()
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("no keychain")
	}

	if err := c.KC.SetIdleTimeout(c.Seconds); err != nil {
		return err
	}
	fmt.Printf("idle-timeout set to %ds\n", c.Seconds)
	return nil
}
