//go:build darwin

package cli

import (
	"strings"
	"testing"

	kfake "github.com/sunakan/op-keychain/internal/keychain/fake"
)

func TestListEmpty(t *testing.T) {
	kc := kfake.New()

	out, _, code, err := runCapture(func() error {
		return (&ListCmd{KC: kc}).Run()
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

func TestListMultipleSortedByRef(t *testing.T) {
	kc := kfake.New()
	storeEntry(kc, "op://vault/z-item/field", "Z Item", "val", "")
	storeEntry(kc, "op://vault/a-item/field", "A Item", "val", "")

	out, _, code, err := runCapture(func() error {
		return (&ListCmd{KC: kc}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	idxA := strings.Index(out, "a-item")
	idxZ := strings.Index(out, "z-item")
	if idxA < 0 || idxZ < 0 {
		t.Fatalf("stdout = %q, both entries expected", out)
	}
	if idxA > idxZ {
		t.Errorf("entries not sorted: a-item appears after z-item in output")
	}
}

func TestListNameEmpty(t *testing.T) {
	kc := kfake.New()
	ref := "op://vault/item/field"
	storeEntry(kc, ref, "", "val", "")

	out, _, code, err := runCapture(func() error {
		return (&ListCmd{KC: kc}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, ref) {
		t.Errorf("stdout = %q, want ref in output", out)
	}
	if strings.Contains(out, "()") {
		t.Errorf("stdout = %q, should not contain empty parens", out)
	}
}

func TestListKeychainNotExists(t *testing.T) {
	kc := kfake.New()
	kc.ExistsVal = false

	out, _, code, err := runCapture(func() error {
		return (&ListCmd{KC: kc}).Run()
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
