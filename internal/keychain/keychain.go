//go:build darwin

// Package keychain provides helpers for macOS Keychain operations.
package keychain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/term"

	"github.com/sunakan/op-keychain/internal/tracing"
)

const keychainKind = "1Password Cache"

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

// Get returns a cached secret for the given account and ref.
// Returns NotFoundError if the keychain does not exist, CacheMissError if no entry is found.
func Get(ctx context.Context, account, ref string) (string, error) {
	_, span := tracing.Tracer().Start(ctx, "Get")
	defer span.End()

	path, err := FilePath()
	if err != nil {
		tracing.SetSpanError(span, err)
		return "", err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		e := &NotFoundError{Path: path}
		tracing.SetSpanError(span, e)
		return "", e
	}

	out, err := exec.CommandContext(ctx, "security", "find-generic-password", "-s", ref, "-a", account, "-w", path).Output() //nolint:gosec // ref and account are user-controlled inputs, not attacker-controlled
	if err != nil {
		return "", &CacheMissError{Ref: ref}
	}

	var entry Entry
	if err := json.Unmarshal([]byte(strings.TrimRight(string(out), "\r\n")), &entry); err != nil {
		tracing.SetSpanError(span, err)
		return "", fmt.Errorf("failed to parse cache entry: %w", err)
	}
	return entry.Value, nil
}

// Set stores a secret in the keychain cache.
func Set(ctx context.Context, account, ref, value string) error {
	_, span := tracing.Tracer().Start(ctx, "Set")
	defer span.End()

	path, err := FilePath()
	if err != nil {
		tracing.SetSpanError(span, err)
		return err
	}

	entry := Entry{
		Ref:      ref,
		ItemName: itemNameFromRef(ref),
		Value:    value,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		tracing.SetSpanError(span, err)
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	out, err := exec.CommandContext(ctx, "security", "add-generic-password", //nolint:gosec // ref, account and path are user-controlled inputs, not attacker-controlled
		"-s", ref,
		"-a", account,
		"-D", keychainKind,
		"-w", string(data),
		path,
	).CombinedOutput()
	if err != nil {
		tracing.SetSpanError(span, err)
		return fmt.Errorf("security add-generic-password: %w: %s", err, out)
	}
	return nil
}

// itemNameFromRef extracts the item name segment from an op:// reference.
// op://Vault/Item/field → "Item"
func itemNameFromRef(ref string) string {
	parts := strings.SplitN(ref, "/", 5)
	if len(parts) >= 4 {
		return parts[3]
	}
	return ref
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
