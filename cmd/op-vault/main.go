//go:build darwin

// Program op-vault caches op:// secrets in macOS Keychain.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/alecthomas/kong"

	"github.com/sunakan/op-vault/internal/cli"
	"github.com/sunakan/op-vault/internal/tracing"
)

// version is set at build time via -ldflags "-X main.version=x.y.z".
var version = "0.0.0"

type CLI struct {
	Version cli.VersionCmd `cmd:"" help:"Print version"`
	Init    cli.InitCmd    `cmd:"" help:"Initialize the keychain"`
	Status  cli.StatusCmd  `cmd:"" help:"Show keychain status and cache entry count"`
	Set     cli.SetCmd     `cmd:"" help:"Cache a secret in the keychain"`
	Clear   cli.ClearCmd   `cmd:"" help:"Remove all cached entries without deleting the keychain"`
	Reset   cli.ResetCmd   `cmd:"" help:"Remove the keychain"`
	Read    cli.ReadCmd    `cmd:"" help:"Get a secret from cache or 1Password"`
}

func main() {
	if len(os.Args) == 1 {
		os.Args = append(os.Args, "--help")
	}
	os.Exit(run())
}

func run() int {
	ctx := context.Background()
	shutdown, err := tracing.Init(ctx, "op-vault", version)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = shutdown(ctx) }()

	ctx, span := tracing.Tracer().Start(ctx, "main")
	defer span.End()

	c := CLI{
		Version: cli.VersionCmd{Version: version},
	}
	kongCtx := kong.Parse(&c,
		kong.Name("op-vault"),
		kong.Description("Cache op:// secrets in macOS Keychain"),
		kong.Exit(func(code int) {
			if code != 0 {
				tracing.SetSpanError(span, errors.New("kong: parse error"))
			}
			span.End()
			_ = shutdown(ctx)
			if code != 0 {
				os.Exit(2)
			}
			os.Exit(0)
		}),
		kong.BindFor[context.Context](ctx),
	)
	if err := kongCtx.Run(); err != nil {
		tracing.SetSpanError(span, err)
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
