//go:build darwin

package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sunakan/op-keychain/internal/keychain"
)

type ClearCmd struct {
	Yes bool              `name:"yes" help:"Skip confirmation prompt"`
	KC  keychain.Keychain `kong:"-"`
}

func (c *ClearCmd) Run() error {
	exists, err := c.KC.Exists()
	if err != nil {
		return err
	}
	if !exists {
		fmt.Println("no keychain")
		return nil
	}

	if !c.Yes {
		tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err != nil {
			return fmt.Errorf("open /dev/tty: %w", err)
		}
		defer tty.Close()

		fmt.Fprint(tty, "Are you sure you want to clear all cache? [y/N]: ")
		scanner := bufio.NewScanner(tty)
		scanner.Scan()
		answer := strings.TrimSpace(scanner.Text())
		if answer != "y" && answer != "Y" {
			return nil
		}
	}

	if err := c.KC.Delete(); err != nil {
		return err
	}
	fmt.Println("cleared all cache")
	return nil
}
