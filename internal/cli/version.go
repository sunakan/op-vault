//go:build darwin

// Package cli implements the CLI subcommand layer
package cli

import "fmt"

// VersionCmd implements the version subcommand
type VersionCmd struct {
	Version string `kong:"-"`
}

// Run prints the version string to stdout
func (c *VersionCmd) Run() error {
	fmt.Println(c.Version)
	return nil
}
