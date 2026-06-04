//go:build darwin

// Package keychain provides helpers for macOS Keychain operations.
package keychain

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultName is the keychain name used when OP_VAULT_NAME is not set.
const DefaultName = "op-vault"

// Name returns OP_VAULT_NAME if set, otherwise DefaultName.
func Name() string {
	if name := os.Getenv("OP_VAULT_NAME"); name != "" {
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
