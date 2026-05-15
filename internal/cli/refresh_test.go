//go:build darwin

package cli

import (
	"strings"
	"testing"

	kfake "github.com/sunakan/op-keychain/internal/keychain/fake"
	opfake "github.com/sunakan/op-keychain/internal/op/fake"
)

func TestRefreshAllSuccess(t *testing.T) {
	kc := kfake.New()
	ref1 := "op://vault/item1/field"
	ref2 := "op://vault/item2/field"
	storeEntry(kc, ref1, "Item1", "old1", "acc")
	storeEntry(kc, ref2, "Item2", "old2", "acc")

	op := opfake.New()
	op.Secrets[ref1] = "new1"
	op.Secrets[ref2] = "new2"
	op.Titles[ref1] = "Item1"
	op.Titles[ref2] = "Item2"

	out, _, code, err := runCapture(func() error {
		return (&RefreshCmd{KC: kc, OP: op}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "done: 2 updated, 0 failed") {
		t.Errorf("stdout = %q, want '2 updated, 0 failed'", out)
	}
}

func TestRefreshPartialFailure(t *testing.T) {
	kc := kfake.New()
	ref1 := "op://vault/item1/field"
	ref2 := "op://vault/item2/field"
	storeEntry(kc, ref1, "Item1", "old1", "acc")
	storeEntry(kc, ref2, "Item2", "old2", "acc")

	op := opfake.New()
	op.Secrets[ref1] = "new1"
	op.Titles[ref1] = "Item1"
	// ref2 not in Secrets → resolve returns error

	out, _, code, err := runCapture(func() error {
		return (&RefreshCmd{KC: kc, OP: op}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "done: 1 updated, 1 failed") {
		t.Errorf("stdout = %q, want '1 updated, 1 failed'", out)
	}
}

func TestRefreshNoCache(t *testing.T) {
	kc := kfake.New()
	op := opfake.New()

	out, _, code, err := runCapture(func() error {
		return (&RefreshCmd{KC: kc, OP: op}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "no cache") {
		t.Errorf("stdout = %q, want 'no cache'", out)
	}
}

func TestRefreshKeychainNotExists(t *testing.T) {
	kc := kfake.New()
	kc.ExistsVal = false
	op := opfake.New()

	out, _, code, err := runCapture(func() error {
		return (&RefreshCmd{KC: kc, OP: op}).Run()
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
