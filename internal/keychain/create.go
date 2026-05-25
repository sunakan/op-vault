//go:build darwin

package keychain

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/sunakan/op-keychain/internal/tracing"
)

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
	if err := cgoCreate(path, password); err != nil {
		tracing.SetSpanError(span, err)
		return err
	}
	return nil
}
