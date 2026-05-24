//go:build darwin

// Program op-keychain caches op:// secrets in macOS Keychain.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"

	"github.com/sunakan/op-keychain/internal/cli"
	"github.com/sunakan/op-keychain/internal/tracing"
)

const version = "0.0.0"

type CLI struct {
	Version cli.VersionCmd `cmd:"" help:"Print version"`
	Init    cli.InitCmd    `cmd:"" help:"Initialize the keychain"`
	Reset   cli.ResetCmd   `cmd:"" help:"Remove the keychain"`
}

func main() {
	if len(os.Args) == 1 {
		os.Args = append(os.Args, "--help")
	}
	os.Exit(run())
}

func run() int {
	ctx := context.Background()
	shutdown, err := tracing.Init(ctx, "op-keychain", version)
	if err != nil {
		slog.Error("failed to initialize tracer", "err", err)
		return 1
	}
	defer func() { _ = shutdown(ctx) }()

	ctx, span := tracing.Tracer().Start(ctx, "main")
	defer span.End()

	c := CLI{
		Version: cli.VersionCmd{Version: version},
	}
	kongCtx := kong.Parse(&c,
		kong.Name("op-keychain"),
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
		slog.Error("command failed", "err", err)
		return 1
	}
	return 0
}
