//go:build darwin

package keychain

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
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

func (k *ExecKeychain) Unlock() error {
	// Step 1: silent attempt with empty password (stderr suppressed to avoid noise)
	cmd := exec.Command("security", "unlock-keychain", "-p", "", k.path)
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err == nil {
		return nil
	}
	// Step 2: let macOS show the GUI dialog
	return exec.Command("security", "unlock-keychain", k.path).Run()
}

func (k *ExecKeychain) SetIdleTimeout(seconds int) error {
	if err := exec.Command("security", "set-keychain-settings", "-t", strconv.Itoa(seconds), k.path).Run(); err != nil {
		return fmt.Errorf("set-keychain-settings: %w", err)
	}
	return nil
}

func (k *ExecKeychain) GetIdleTimeout() (int, error) {
	// show-keychain-info outputs to stderr on macOS
	out, err := exec.Command("security", "show-keychain-info", k.path).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("show-keychain-info: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		i := strings.Index(line, "timeout=")
		if i < 0 {
			continue
		}
		rest := line[i+len("timeout="):]
		end := 0
		for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
			end++
		}
		if end > 0 {
			if n, err := strconv.Atoi(rest[:end]); err == nil {
				return n, nil
			}
		}
	}
	return 0, fmt.Errorf("timeout not found in keychain info")
}

func (k *ExecKeychain) ListServices() ([]string, error) {
	out, err := exec.Command("security", "dump-keychain", k.path).Output()
	if err != nil {
		return nil, fmt.Errorf("dump-keychain: %w", err)
	}
	const marker = `="op-keychain:`
	var services []string
	for _, line := range strings.Split(string(out), "\n") {
		i := strings.Index(line, marker)
		if i < 0 {
			continue
		}
		// line[i:] looks like `="op-keychain:abc..."`; skip `="` to get `op-keychain:abc..."`
		rest := line[i+2:]
		j := strings.Index(rest, `"`)
		if j > 0 {
			services = append(services, rest[:j])
		}
	}
	return services, nil
}

// Get, Set, Remove は Step 6a で実装する
func (k *ExecKeychain) Get(service string) (string, error) { panic("not implemented") }
func (k *ExecKeychain) Set(service, value string) error    { panic("not implemented") }
func (k *ExecKeychain) Remove(service string) error        { panic("not implemented") }
