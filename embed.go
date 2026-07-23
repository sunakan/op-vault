//go:build darwin

// Package opvault embeds documentation shipped alongside the module.
package opvault

import _ "embed"

// Readme is the content of README.md
//
//go:embed README.md
var Readme string
