//go:build darwin

// Package keychain provides helpers for macOS Keychain operations.
package keychain

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/term"

	"github.com/sunakan/op-keychain/internal/tracing"
)

// DefaultName is the keychain name used when OP_KEYCHAIN_NAME is not set.
const DefaultName = "op-keychain"

// Name returns OP_KEYCHAIN_NAME if set, otherwise DefaultName.
func Name() string {
	if name := os.Getenv("OP_KEYCHAIN_NAME"); name != "" {
		return name
	}
	return DefaultName
}

// FilePath returns the full path to the keychain file.
func FilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, "Library", "Keychains", Name()+".keychain-db"), nil
}

// ReadPassword reads a password from stdin.
// If stdin is a TTY, it prompts on stderr with no echo; the OTel span duration includes user input time.
// Otherwise, it reads from stdin directly (e.g. echo 'password' | op-keychain init).
func ReadPassword(ctx context.Context) (string, error) {
	_, span := tracing.Tracer().Start(ctx, "ReadPassword")
	defer span.End()
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stderr, "Keychain password: ")
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		return string(b), err
	}
	b, err := io.ReadAll(os.Stdin)
	return strings.TrimRight(string(b), "\r\n"), err
}

// Create creates a new keychain at the given path with the given password.
func Create(ctx context.Context, path, password string) error {
	_, span := tracing.Tracer().Start(ctx, "Create")
	defer span.End()
	out, err := exec.CommandContext(ctx, "security", "create-keychain", "-p", password, path).CombinedOutput() //nolint:gosec // path and password are user-controlled inputs, not attacker-controlled
	if err != nil {
		tracing.SetSpanError(span, err)
		return fmt.Errorf("security create-keychain: %w: %s", err, out)
	}
	return nil
}

// Remove removes a keychain at the given path.
func Remove(ctx context.Context, path string) error {
	_, span := tracing.Tracer().Start(ctx, "Remove")
	defer span.End()
	out, err := exec.CommandContext(ctx, "security", "delete-keychain", path).CombinedOutput() //nolint:gosec // path is a user-controlled input, not attacker-controlled
	if err != nil {
		tracing.SetSpanError(span, err)
		return fmt.Errorf("security delete-keychain: %w: %s", err, out)
	}
	return nil
}
