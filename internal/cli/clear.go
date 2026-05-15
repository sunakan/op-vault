//go:build darwin

package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sunakan/op-keychain/internal/keychain"
)

type ClearCmd struct {
	Yes   bool              `name:"yes" help:"Skip confirmation prompt"`
	KC    keychain.Keychain `kong:"-"`
	Input io.Reader         `kong:"-"`
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
		var reader io.Reader
		if c.Input != nil {
			reader = c.Input
		} else {
			input, cleanup, err := openInputFile()
			if err != nil {
				return err
			}
			defer cleanup()
			reader = input
		}

		fmt.Fprint(os.Stderr, "Are you sure you want to clear all cache? [y/N]: ")
		scanner := bufio.NewScanner(reader)
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
