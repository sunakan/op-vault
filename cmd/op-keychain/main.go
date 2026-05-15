//go:build darwin

package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/sunakan/op-keychain/internal/cli"
	"github.com/sunakan/op-keychain/internal/keychain"
	"github.com/sunakan/op-keychain/internal/logging"
)

var version = "dev"

type CLI struct {
	Read           cli.ReadCmd           `cmd:"" help:"Read a secret with cache"`
	Remove         cli.RemoveCmd         `cmd:"" help:"Remove a cached entry"`
	Clear          cli.ClearCmd          `cmd:"" help:"Clear all cache"`
	List           cli.ListCmd           `cmd:"" help:"List cached entries"`
	Refresh        cli.RefreshCmd        `cmd:"" help:"Refresh all entries from 1Password"`
	Status         cli.StatusCmd         `cmd:"" help:"Show keychain status"`
	SetIdleTimeout cli.SetIdleTimeoutCmd `cmd:"" name:"set-idle-timeout" help:"Set auto-lock timeout"`
	Init           cli.InitCmd           `cmd:"" help:"Initialize the keychain"`
	Version        cli.VersionCmd        `cmd:"" help:"Print version"`
}

func main() {
	logging.Init()
	kc := keychain.NewExecKeychain()
	var cliCmd CLI
	cliCmd.Version.Version = version
	cliCmd.Read.KC = kc
	cliCmd.Read.AppVersion = version
	cliCmd.Refresh.KC = kc
	cliCmd.Refresh.AppVersion = version
	cliCmd.Init.KC = kc
	cliCmd.Clear.KC = kc
	cliCmd.Status.KC = kc
	cliCmd.SetIdleTimeout.KC = kc
	cliCmd.List.KC = kc
	cliCmd.Remove.KC = kc
	ctx := kong.Parse(&cliCmd,
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
