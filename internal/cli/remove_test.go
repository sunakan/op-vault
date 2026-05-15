//go:build darwin

package cli

import (
	"strings"
	"testing"

	"github.com/sunakan/op-keychain/internal/keychain"
	kfake "github.com/sunakan/op-keychain/internal/keychain/fake"
)

func TestRemoveEntryExists(t *testing.T) {
	kc := kfake.New()
	ref := "op://vault/item/field"
	storeEntry(kc, ref, "Item", "val", "")

	out, _, code, err := runCapture(func() error {
		return (&RemoveCmd{Ref: ref, KC: kc}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, ref) {
		t.Errorf("stdout = %q, want it to contain ref", out)
	}
	if _, ok := kc.Entries[keychain.Service(ref)]; ok {
		t.Error("entry still in keychain after remove")
	}
}

func TestRemoveEntryNotFound(t *testing.T) {
	kc := kfake.New()
	ref := "op://vault/item/field"

	_, errOut, code, _ := runCapture(func() error {
		return (&RemoveCmd{Ref: ref, KC: kc}).Run()
	})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "cache not found") {
		t.Errorf("stderr = %q, want 'cache not found'", errOut)
	}
}

func TestRemoveKeychainNotExists(t *testing.T) {
	kc := kfake.New()
	kc.ExistsVal = false
	ref := "op://vault/item/field"

	_, errOut, code, _ := runCapture(func() error {
		return (&RemoveCmd{Ref: ref, KC: kc}).Run()
	})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "no keychain") {
		t.Errorf("stderr = %q, want 'no keychain'", errOut)
	}
}

func TestRemoveLockedUnlockRetry(t *testing.T) {
	kc := kfake.New()
	kc.Locked = true
	ref := "op://vault/item/field"
	storeEntry(kc, ref, "Item", "val", "")

	out, _, code, err := runCapture(func() error {
		return (&RemoveCmd{Ref: ref, KC: kc}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "removed") {
		t.Errorf("stdout = %q, want 'removed'", out)
	}
}

func TestRemoveInvalidRef(t *testing.T) {
	kc := kfake.New()
	_, _, code, _ := runCapture(func() error {
		return (&RemoveCmd{Ref: "not-a-ref", KC: kc}).Run()
	})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}
