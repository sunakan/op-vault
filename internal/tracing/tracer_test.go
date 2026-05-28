//go:build darwin

package tracing

import "testing"

func TestExtractImportPath(t *testing.T) {
	tests := []struct {
		fullName string
		expected string
	}{
		{
			fullName: "github.com/sunakan/op-keychain/internal/cli.(*VersionCmd).Run",
			expected: "github.com/sunakan/op-keychain/internal/cli",
		},
		{
			fullName: "github.com/sunakan/op-keychain/internal/tracing.Tracer",
			expected: "github.com/sunakan/op-keychain/internal/tracing",
		},
		{
			fullName: "main.main",
			expected: "main",
		},
		{
			fullName: "unknownFormat",
			expected: "unknownFormat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.fullName, func(t *testing.T) {
			got := extractImportPath(tt.fullName)
			if got != tt.expected {
				t.Errorf("extractImportPath(%q) = %q; want %q", tt.fullName, got, tt.expected)
			}
		})
	}
}
