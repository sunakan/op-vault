//go:build darwin

package cli

import (
	"fmt"

	"github.com/sunakan/op-keychain/internal/keychain"
)

type StatusCmd struct {
	KC keychain.Keychain `kong:"-"`
}

func (c *StatusCmd) Run() error {
	exists, err := c.KC.Exists()
	if err != nil {
		return err
	}
	if !exists {
		fmt.Println("keychain: none")
		return nil
	}

	// IsLocked を先に呼ぶ: GetIdleTimeout (security show-keychain-info) はロック中に
	// SecurityAgent ダイアログを出すため、ロック確認後に呼ぶ必要がある
	locked, err := c.KC.IsLocked()
	if err != nil {
		return err
	}

	fmt.Printf("keychain:     %s\n", c.KC.Path())

	if locked {
		fmt.Println("idle-timeout: locked (unlock to view)")
		fmt.Println("lock status:  locked")
		fmt.Println("entries:      unknown (locked)")
		return nil
	}

	timeout, err := c.KC.GetIdleTimeout()
	if err != nil {
		return err
	}

	services, err := c.KC.ListServices()
	if err != nil {
		return err
	}
	fmt.Printf("idle-timeout: %ds\n", timeout)
	fmt.Println("lock status:  unlocked")
	fmt.Printf("entries:      %d\n", len(services))
	return nil
}
