//go:build darwin

package cli

import (
	"context"
	"fmt"

	"github.com/sunakan/op-keychain/internal/keychain"
	"github.com/sunakan/op-keychain/internal/tracing"
)

// StatusCmd implements the status subcommand.
type StatusCmd struct{}

// Run prints the current state of the keychain to stdout.
func (c *StatusCmd) Run(ctx context.Context) error {
	_, span := tracing.Tracer().Start(ctx, "status")
	defer span.End()

	result, err := keychain.Status(ctx)
	if err != nil {
		tracing.SetSpanError(span, err)
		return err
	}

	if !result.Initialized {
		fmt.Println("status: not initialized")
		return nil
	}

	if !result.Unlocked {
		fmt.Println("status: locked")
		fmt.Printf("path: %s\n", result.Path)
		return nil
	}

	noun := "entries"
	if result.EntryCount == 1 {
		noun = "entry"
	}
	fmt.Println("status: unlocked")
	fmt.Printf("path: %s\n", result.Path)
	fmt.Printf("cache: %d %s\n", result.EntryCount, noun)
	// SecKeychainCopySettings always returns useLockInterval=false on modern macOS;
	// use LockInterval to determine the display value directly.
	// 0x7fffffff is the sentinel written by `security set-keychain-settings` (no -t)
	// meaning no timeout; treat both 0 and sentinel as "never".
	const noLockInterval = uint32(0x7fffffff)
	switch {
	case result.LockInterval == 0 || result.LockInterval == noLockInterval:
		fmt.Println("auto-lock: never")
	case result.LockInterval%60 == 0:
		fmt.Printf("auto-lock: %d minutes\n", result.LockInterval/60)
	default:
		fmt.Printf("auto-lock: %d seconds\n", result.LockInterval)
	}
	if result.LockOnSleep {
		fmt.Println("lock-on-sleep: true")
	} else {
		fmt.Println("lock-on-sleep: false")
	}
	return nil
}
