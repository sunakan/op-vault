//go:build darwin

package cli

import (
	"strings"
	"testing"
)

func TestVersionPrint(t *testing.T) {
	out, _, code, err := runCapture(func() error {
		return (&VersionCmd{Version: "1.2.3"}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "1.2.3") {
		t.Errorf("stdout = %q, want version '1.2.3'", out)
	}
}
