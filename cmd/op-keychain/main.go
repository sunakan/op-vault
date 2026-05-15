//go:build darwin

package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/sunakan/op-keychain/internal/cli"
	"github.com/sunakan/op-keychain/internal/keychain"
	"github.com/sunakan/op-keychain/internal/op"
)

var version = "dev"

type CLI struct {
	Read           ReadCmd           `cmd:"" help:"Read a secret with cache"`
	Remove         RemoveCmd         `cmd:"" help:"Remove a cached entry"`
	Clear          cli.ClearCmd      `cmd:"" help:"Clear all cache"`
	List           ListCmd           `cmd:"" help:"List cached entries"`
	Refresh        RefreshCmd        `cmd:"" help:"Refresh all entries from 1Password"`
	Status         StatusCmd         `cmd:"" help:"Show keychain status"`
	SetIdleTimeout SetIdleTimeoutCmd `cmd:"" name:"set-idle-timeout" help:"Set auto-lock timeout"`
	Init           cli.InitCmd       `cmd:"" help:"Initialize the keychain"`
	Version        cli.VersionCmd    `cmd:"" help:"Print version"`
}

type ReadCmd struct {
	Ref     string `arg:"" help:"op://vault/item[/field]"`
	Account string `short:"a" name:"account" optional:"" env:"OP_ACCOUNT" help:"1Password account name"`
}

func (c *ReadCmd) Run() error {
	if _, err := op.ParseRef(c.Ref); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	fmt.Println("not implemented")
	return nil
}

type RemoveCmd struct {
	Ref string `arg:"" help:"op://vault/item[/field]"`
}

func (c *RemoveCmd) Run() error {
	if _, err := op.ParseRef(c.Ref); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	fmt.Println("not implemented")
	return nil
}


type ListCmd struct{}

func (c *ListCmd) Run() error {
	fmt.Println("not implemented")
	return nil
}

type RefreshCmd struct {
	Account string `short:"a" name:"account" optional:"" env:"OP_ACCOUNT" help:"1Password account name"`
}

func (c *RefreshCmd) Run() error {
	fmt.Println("not implemented")
	return nil
}

type StatusCmd struct{}

func (c *StatusCmd) Run() error {
	fmt.Println("not implemented")
	return nil
}

type SetIdleTimeoutCmd struct {
	Seconds int `arg:"" help:"Timeout in seconds (positive integer)"`
}

func (c *SetIdleTimeoutCmd) Run() error {
	fmt.Println("not implemented")
	return nil
}

func main() {
	kc := keychain.NewExecKeychain()
	var cliCmd CLI
	cliCmd.Version.Version = version
	cliCmd.Init.KC = kc
	cliCmd.Clear.KC = kc
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
