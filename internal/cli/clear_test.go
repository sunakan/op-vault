//go:build darwin

package cli

import (
	"strings"
	"testing"

	kfake "github.com/sunakan/op-keychain/internal/keychain/fake"
)

func TestClearWithYesFlag(t *testing.T) {
	kc := kfake.New()
	storeEntry(kc, "op://vault/item/field", "Item", "val", "")

	out, _, code, err := runCapture(func() error {
		return (&ClearCmd{Yes: true, KC: kc}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "cleared all cache") {
		t.Errorf("stdout = %q, want 'cleared all cache'", out)
	}
	if kc.ExistsVal {
		t.Error("keychain still exists after clear")
	}
}

func TestClearWithYInput(t *testing.T) {
	kc := kfake.New()
	storeEntry(kc, "op://vault/item/field", "Item", "val", "")

	out, _, code, err := runCapture(func() error {
		return (&ClearCmd{KC: kc, Input: strings.NewReader("y\n")}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "cleared all cache") {
		t.Errorf("stdout = %q, want 'cleared all cache'", out)
	}
}

func TestClearWithNInput(t *testing.T) {
	kc := kfake.New()
	storeEntry(kc, "op://vault/item/field", "Item", "val", "")

	out, _, code, err := runCapture(func() error {
		return (&ClearCmd{KC: kc, Input: strings.NewReader("N\n")}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.Contains(out, "cleared all cache") {
		t.Errorf("stdout = %q, should not contain 'cleared all cache' after N", out)
	}
	if !kc.ExistsVal {
		t.Error("keychain deleted after N answer")
	}
}

func TestClearKeychainNotExists(t *testing.T) {
	kc := kfake.New()
	kc.ExistsVal = false

	out, _, code, err := runCapture(func() error {
		return (&ClearCmd{Yes: true, KC: kc}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "no keychain") {
		t.Errorf("stdout = %q, want 'no keychain'", out)
	}
}
