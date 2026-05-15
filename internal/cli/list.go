//go:build darwin

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/sunakan/op-keychain/internal/keychain"
)

type ListCmd struct {
	KC keychain.Keychain `kong:"-"`
}

func (c *ListCmd) Run() error {
	exists, err := c.KC.Exists()
	if err != nil {
		return err
	}
	if !exists {
		fmt.Println("no keychain")
		return nil
	}

	locked, err := c.KC.IsLocked()
	if err != nil {
		return err
	}
	if locked {
		if err := c.KC.Unlock(); err != nil {
			return err
		}
	}

	services, err := c.KC.ListServices()
	if err != nil {
		return err
	}
	if len(services) == 0 {
		fmt.Println("no cache")
		return nil
	}

	var entries []keychain.Entry
	for _, svc := range services {
		raw, err := c.KC.Get(svc)
		if err != nil {
			continue
		}
		var e keychain.Entry
		if err := json.Unmarshal([]byte(raw), &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}

	if len(entries) == 0 {
		fmt.Println("no cache")
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Ref < entries[j].Ref
	})

	for _, e := range entries {
		if e.Name != "" {
			fmt.Printf("  %s (%s)\n", e.Name, e.Ref)
		} else {
			fmt.Printf("  %s\n", e.Ref)
		}
	}
	return nil
}
