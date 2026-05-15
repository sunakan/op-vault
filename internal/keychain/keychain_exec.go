//go:build darwin

package keychain

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

type ExecKeychain struct {
	name string
	path string
}

func NewExecKeychain() *ExecKeychain {
	name := os.Getenv("OP_KEYCHAIN_NAME")
	if name == "" {
		name = "op-keychain"
	}
	u, _ := user.Current()
	path := filepath.Join(u.HomeDir, "Library", "Keychains", name+".keychain-db")
	return &ExecKeychain{name: name, path: path}
}

func (k *ExecKeychain) Path() string {
	return k.path
}

func (k *ExecKeychain) Exists() (bool, error) {
	_, err := os.Stat(k.path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func (k *ExecKeychain) Create(password string, idleTimeout time.Duration) error {
	if err := exec.Command("security", "create-keychain", "-p", password, k.name+".keychain").Run(); err != nil {
		return fmt.Errorf("create-keychain: %w", err)
	}
	seconds := int(idleTimeout.Seconds())
	if err := exec.Command("security", "set-keychain-settings", "-t", fmt.Sprintf("%d", seconds), k.path).Run(); err != nil {
		return fmt.Errorf("set-keychain-settings: %w", err)
	}
	return nil
}

func (k *ExecKeychain) Delete() error {
	if err := exec.Command("security", "delete-keychain", k.path).Run(); err != nil {
		return fmt.Errorf("delete-keychain: %w", err)
	}
	return nil
}

func (k *ExecKeychain) AddToList() error {
	out, err := exec.Command("security", "list-keychains", "-d", "user").Output()
	if err != nil {
		return fmt.Errorf("list-keychains: %w", err)
	}
	var existing []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		trimmed := strings.Trim(strings.TrimSpace(line), `"`)
		if trimmed != "" {
			existing = append(existing, trimmed)
		}
	}
	args := append([]string{"list-keychains", "-d", "user", "-s"}, existing...)
	args = append(args, k.path)
	if err := exec.Command("security", args...).Run(); err != nil {
		return fmt.Errorf("list-keychains -s: %w", err)
	}
	return nil
}

// 残りのメソッドは Step 5a・6a で実装する
func (k *ExecKeychain) Get(service string) (string, error)      { panic("not implemented") }
func (k *ExecKeychain) Set(service, value string) error         { panic("not implemented") }
func (k *ExecKeychain) Remove(service string) error             { panic("not implemented") }
func (k *ExecKeychain) ListServices() ([]string, error)         { panic("not implemented") }
func (k *ExecKeychain) Unlock() error                           { panic("not implemented") }
func (k *ExecKeychain) SetIdleTimeout(seconds int) error        { panic("not implemented") }
func (k *ExecKeychain) GetIdleTimeout() (int, error)            { panic("not implemented") }
func (k *ExecKeychain) IsLocked() (bool, error)                 { panic("not implemented") }
