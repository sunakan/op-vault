//go:build darwin

package cli

// Package cli implements the CLI subcommand layer.
import "fmt"

type VersionCmd struct {
	Version string `kong:"-"`
}

func (c *VersionCmd) Run() error {
	fmt.Println(c.Version)
	return nil
}
