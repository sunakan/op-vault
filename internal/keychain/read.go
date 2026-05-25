//go:build darwin

package keychain

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sunakan/op-keychain/internal/tracing"
)

const keychainKind = "1Password Cache"

// Entry represents a cached 1Password secret.
type Entry struct {
	Ref      string `json:"ref"`
	ItemName string `json:"item_name"`
	Value    string `json:"value"`
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

	// exec instead of CGO: `security find-generic-password -w` outputs the secret via stdout,
	// not as a command-line arg, so there is no ps exposure.
	// CGO SecItemCopyMatching prompts for permission when reading items created by other apps
	// (e.g. `security add-generic-password` in E2E setup), so exec avoids the dialog entirely.
	out, err := exec.CommandContext(ctx, "security", "find-generic-password", "-s", ref, "-a", account, "-w", path).Output() //nolint:gosec // ref, account, path are user-controlled, not attacker-controlled
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

	if err := cgoAdd(path, ref, account, keychainKind, data); err != nil {
		tracing.SetSpanError(span, err)
		return err
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
