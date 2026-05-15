//go:build darwin

package cli

import "fmt"

type VersionCmd struct {
	Version string `kong:"-"`
}

func (c *VersionCmd) Run() error {
	fmt.Printf("op-keychain %s\n", c.Version)
	return nil
}
