//go:build darwin

package keychain

import (
	"encoding/hex"
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
	name     string
	path     string
	username string
}

func NewExecKeychain() *ExecKeychain {
	name := os.Getenv("OP_KEYCHAIN_NAME")
	if name == "" {
		name = "op-keychain"
	}
	u, _ := user.Current()
	path := filepath.Join(u.HomeDir, "Library", "Keychains", name+".keychain-db")
	return &ExecKeychain{name: name, path: path, username: u.Username}
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
	// "svce"<blob>="op-keychain:..." の行だけを対象にする。
	// dump-keychain は同じ service 名を 0x00000007 行と "svce" 行の2行に出力するため、
	// "svce" に絞らないと重複が生じる。
	const marker = `"svce"<blob>="op-keychain:`
	var services []string
	for _, line := range strings.Split(string(out), "\n") {
		i := strings.Index(line, marker)
		if i < 0 {
			continue
		}
		// `"svce"<blob>="op-keychain:abc..."` の `"op-keychain:` 以降を取り出す
		rest := line[i+len(`"svce"<blob>="`):] // op-keychain:abc..."
		j := strings.Index(rest, `"`)
		if j > 0 {
			services = append(services, rest[:j])
		}
	}
	return services, nil
}

func (k *ExecKeychain) Get(service string) (string, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-s", service, "-a", k.username, "-w", k.path).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 44 {
			return "", ErrNotFound
		}
		return "", ErrLocked
	}
	raw := strings.TrimRight(string(out), "\n")
	if strings.HasPrefix(raw, "0x") {
		decoded, decErr := hex.DecodeString(raw[2:])
		if decErr != nil {
			return "", fmt.Errorf("hex decode: %w", decErr)
		}
		return string(decoded), nil
	}
	return raw, nil
}

func (k *ExecKeychain) Set(service, value string) error {
	err := exec.Command("security", "add-generic-password",
		"-U", "-s", service, "-a", k.username, "-w", value, k.path).Run()
	if err != nil {
		return ErrLocked
	}
	return nil
}

func (k *ExecKeychain) Remove(service string) error {
	err := exec.Command("security", "delete-generic-password",
		"-s", service, "-a", k.username, k.path).Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 44 {
			return ErrNotFound
		}
		return ErrLocked
	}
	return nil
}
