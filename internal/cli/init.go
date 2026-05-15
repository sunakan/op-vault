//go:build darwin

package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sunakan/op-keychain/internal/keychain"
	"golang.org/x/term"
)

type InitCmd struct {
	KC keychain.Keychain `kong:"-"`
}

func (c *InitCmd) Run() error {
	exists, err := c.KC.Exists()
	if err != nil {
		return err
	}
	if exists {
		fmt.Println("already initialized")
		return nil
	}
	return initInteractive(c.KC)
}

// AutoCreate creates a keychain with empty password and no prompt.
// Used by read when the keychain doesn't exist yet.
func AutoCreate(kc keychain.Keychain) error {
	return createKeychain(kc, "")
}

func initInteractive(kc keychain.Keychain) error {
	input, cleanup, err := openInputFile()
	if err != nil {
		return err
	}
	defer cleanup()

	fmt.Fprint(os.Stderr, "Set a password for the keychain? [y/N]: ")
	scanner := bufio.NewScanner(input)
	scanner.Scan()
	answer := strings.TrimSpace(scanner.Text())

	if answer != "y" && answer != "Y" {
		return createKeychain(kc, "")
	}

	fmt.Fprint(os.Stderr, "Enter password: ")
	pw1, err := term.ReadPassword(int(input.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}

	fmt.Fprint(os.Stderr, "Confirm password: ")
	pw2, err := term.ReadPassword(int(input.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}

	if string(pw1) != string(pw2) {
		fmt.Fprintln(os.Stderr, "error: passwords do not match")
		os.Exit(1)
	}

	return createKeychain(kc, string(pw1))
}

// openInputFile returns stdin if it is a terminal (works with expect),
// otherwise falls back to /dev/tty (works when called from a subshell).
func openInputFile() (f *os.File, cleanup func(), err error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return os.Stdin, func() {}, nil
	}
	f, err = os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("open /dev/tty: %w", err)
	}
	return f, func() { _ = f.Close() }, nil
}

func createKeychain(kc keychain.Keychain, password string) error {
	if err := kc.Create(password, idleTimeout()); err != nil {
		return err
	}
	return kc.AddToList()
}

func idleTimeout() time.Duration {
	val := os.Getenv("OP_KEYCHAIN_IDLE_TIMEOUT")
	if val == "" {
		return 3600 * time.Second
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return 3600 * time.Second
	}
	return time.Duration(n) * time.Second
}
