//go:build darwin

// Program op-keychain caches op:// secrets in macOS Keychain.
package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

const version = "dev"

type CLI struct {
	Version VersionCmd `cmd:"" help:"Print version"`
}

type VersionCmd struct{}

func (c *VersionCmd) Run() error {
	fmt.Printf("op-keychain %s\n", version)
	return nil
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("op-keychain"),
		kong.Description("Cache op:// secrets in macOS Keychain"),
	)
	if err := ctx.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
