//go:build darwin

// Program op-keychain caches op:// secrets in macOS Keychain.
package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

const version = "0.0.0"

type CLI struct {
	Version VersionCmd `cmd:"" help:"Print version"`
}

type VersionCmd struct{}

func (c *VersionCmd) Run() error {
	fmt.Println(version)
	return nil
}

func main() {
	if len(os.Args) == 1 {
		os.Args = append(os.Args, "--help")
	}
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("op-keychain"),
		kong.Description("Cache op:// secrets in macOS Keychain"),
		kong.Exit(func(code int) {
			if code != 0 {
				os.Exit(2)
			}
			os.Exit(0)
		}),
	)
	if err := ctx.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
