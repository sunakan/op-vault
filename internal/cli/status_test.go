//go:build darwin

package cli

import (
	"strings"
	"testing"

	kfake "github.com/sunakan/op-keychain/internal/keychain/fake"
)

func TestStatusKeychainNotExists(t *testing.T) {
	kc := kfake.New()
	kc.ExistsVal = false

	out, _, code, err := runCapture(func() error {
		return (&StatusCmd{KC: kc}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "none") {
		t.Errorf("stdout = %q, want 'none'", out)
	}
}

func TestStatusUnlocked(t *testing.T) {
	kc := kfake.New()
	kc.Locked = false
	kc.IdleTimeout = 3600
	storeEntry(kc, "op://vault/item/field", "Item", "val", "")

	out, _, code, err := runCapture(func() error {
		return (&StatusCmd{KC: kc}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "unlocked") {
		t.Errorf("stdout = %q, want 'unlocked'", out)
	}
	if !strings.Contains(out, "3600s") {
		t.Errorf("stdout = %q, want '3600s'", out)
	}
	if !strings.Contains(out, "entries:") {
		t.Errorf("stdout = %q, want 'entries:'", out)
	}
}

func TestStatusLocked(t *testing.T) {
	kc := kfake.New()
	kc.Locked = true

	out, _, code, err := runCapture(func() error {
		return (&StatusCmd{KC: kc}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "locked") {
		t.Errorf("stdout = %q, want 'locked'", out)
	}
	if !strings.Contains(out, "unlock to view") {
		t.Errorf("stdout = %q, want 'unlock to view'", out)
	}
}
