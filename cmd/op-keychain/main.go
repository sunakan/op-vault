//go:build darwin

// Program op-keychain caches op:// secrets in macOS Keychain.
package main

import (
	"log/slog"
	"os"

	"github.com/sunakan/op-keychain/internal/cli"

	"github.com/alecthomas/kong"
)

const version = "0.0.0"

type CLI struct {
	Version cli.VersionCmd `cmd:"" help:"Print version"`
}

func main() {
	if len(os.Args) == 1 {
		os.Args = append(os.Args, "--help")
	}
	c := CLI{
		Version: cli.VersionCmd{Version: version},
	}
	ctx := kong.Parse(&c,
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
		slog.Error("command failed", "err", err)
		os.Exit(1)
	}
}
