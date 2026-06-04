//go:build darwin

package keychain

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// newTempKeychain creates a keychain in t.TempDir() and registers cleanup via cgoDelete.
func newTempKeychain(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.keychain-db")
	if err := cgoCreate(path, "testpass"); err != nil {
		t.Fatalf("cgoCreate: %v", err)
	}
	t.Cleanup(func() { _ = cgoDelete(path) })
	return path
}

func TestCgoCreate(t *testing.T) {
	// newTempKeychain calls cgoCreate internally, so TestCgoCreate uses
	// t.TempDir() + t.Cleanup(cgoDelete) directly to avoid recursive dependency.
	t.Run("success", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "test.keychain-db")
		if err := cgoCreate(path, "testpass"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Cleanup(func() { _ = cgoDelete(path) })
	})

	t.Run("empty password", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "test.keychain-db")
		if err := cgoCreate(path, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Cleanup(func() { _ = cgoDelete(path) })
	})

	t.Run("duplicate path returns error", func(t *testing.T) {
		path := newTempKeychain(t)
		if err := cgoCreate(path, "testpass"); err == nil {
			t.Fatal("expected error for duplicate keychain, got nil")
		}
	})
}

func TestCgoDelete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "test.keychain-db")
		if err := cgoCreate(path, "testpass"); err != nil {
			t.Fatalf("cgoCreate: %v", err)
		}
		if err := cgoDelete(path); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("expected keychain file to be deleted")
		}
	})

	t.Run("nonexistent path returns error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nonexistent.keychain-db")
		if err := cgoDelete(path); err == nil {
			t.Fatal("expected error for nonexistent keychain, got nil")
		}
	})
}

func TestCgoAdd(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		path := newTempKeychain(t)
		ref, account := "op://V/Item/f", "acct"
		want := []byte(`"secret"`)
		if err := cgoAdd(path, ref, account, keychainKind, ref, want); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, found, err := cgoGet(path, ref, account)
		if err != nil {
			t.Fatalf("cgoGet: %v", err)
		}
		if !found {
			t.Fatal("expected item to be retrievable after add")
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("empty data", func(t *testing.T) {
		path := newTempKeychain(t)
		ref, account := "op://V/Item/f", "acct"
		if err := cgoAdd(path, ref, account, keychainKind, ref, []byte{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, found, err := cgoGet(path, ref, account)
		if err != nil {
			t.Fatalf("cgoGet: %v", err)
		}
		if !found {
			t.Fatal("expected found=true after adding empty data")
		}
		if len(got) != 0 {
			t.Fatalf("got %q, want empty", got)
		}
	})

	// NOTE: cgoAdd on a nonexistent keychain path does NOT return an error.
	// SecKeychainOpen is lazy (always returns noErr), and SecItemAdd silently
	// falls back to the login keychain. Intentionally not tested to avoid
	// polluting the user's login keychain.

	t.Run("upsert: second add overwrites value", func(t *testing.T) {
		path := newTempKeychain(t)
		ref, account := "op://V/Item/f", "acct"

		if err := cgoAdd(path, ref, account, keychainKind, ref, []byte(`"first"`)); err != nil {
			t.Fatalf("first add: %v", err)
		}
		if err := cgoAdd(path, ref, account, keychainKind, ref, []byte(`"second"`)); err != nil {
			t.Fatalf("second add (upsert): %v", err)
		}

		data, found, err := cgoGet(path, ref, account)
		if err != nil {
			t.Fatalf("cgoGet: %v", err)
		}
		if !found {
			t.Fatal("expected item to be found after upsert")
		}
		if string(data) != `"second"` {
			t.Fatalf("got %q, want %q", string(data), `"second"`)
		}
	})
}

func TestCgoGet(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		path := newTempKeychain(t)
		ref, account := "op://V/Item/f", "acct"
		want := []byte(`"secret"`)

		if err := cgoAdd(path, ref, account, keychainKind, ref, want); err != nil {
			t.Fatalf("cgoAdd: %v", err)
		}

		got, found, err := cgoGet(path, ref, account)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("expected found=true")
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("not found", func(t *testing.T) {
		path := newTempKeychain(t)
		_, found, err := cgoGet(path, "op://V/Missing/f", "acct")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Fatal("expected found=false")
		}
	})

	t.Run("nonexistent keychain returns not found", func(t *testing.T) {
		// SecKeychainOpen is lazy (always noErr), so cgoGet returns found=false
		// via errSecItemNotFound rather than an error.
		path := filepath.Join(t.TempDir(), "nonexistent.keychain-db")
		_, found, err := cgoGet(path, "op://V/Item/f", "acct")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Fatal("expected found=false for nonexistent keychain")
		}
	})

	t.Run("locked empty-password keychain unlocks silently", func(t *testing.T) {
		// Verifies that SecKeychainUnlock(empty) in kcGet silently unlocks a
		// no-password keychain without triggering a macOS dialog.
		path := filepath.Join(t.TempDir(), "test.keychain-db")
		if err := cgoCreate(path, ""); err != nil {
			t.Fatalf("cgoCreate: %v", err)
		}
		t.Cleanup(func() { _ = cgoDelete(path) })

		ref, account := "op://V/Item/f", "acct"
		want := []byte(`"secret"`)
		if err := cgoAdd(path, ref, account, keychainKind, ref, want); err != nil {
			t.Fatalf("cgoAdd: %v", err)
		}

		if out, err := exec.Command("security", "lock-keychain", path).CombinedOutput(); err != nil {
			t.Fatalf("security lock-keychain: %v: %s", err, out)
		}

		got, found, err := cgoGet(path, ref, account)
		if err != nil {
			t.Fatalf("unexpected error after silent unlock: %v", err)
		}
		if !found {
			t.Fatal("expected found=true after silent unlock")
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("different account returns not found", func(t *testing.T) {
		path := newTempKeychain(t)
		ref := "op://V/Item/f"

		if err := cgoAdd(path, ref, "account-a", keychainKind, ref, []byte(`"secret"`)); err != nil {
			t.Fatalf("cgoAdd: %v", err)
		}

		_, found, err := cgoGet(path, ref, "account-b")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Fatal("expected found=false for different account")
		}
	})
}

func TestCgoGetStatus(t *testing.T) {
	t.Run("newly created keychain is unlocked", func(t *testing.T) {
		path := newTempKeychain(t)
		unlocked, err := cgoGetStatus(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !unlocked {
			t.Fatal("expected newly created keychain to be unlocked")
		}
	})

	t.Run("locked keychain returns unlocked=false", func(t *testing.T) {
		path := newTempKeychain(t)
		if out, err := exec.Command("security", "lock-keychain", path).CombinedOutput(); err != nil {
			t.Fatalf("security lock-keychain: %v: %s", err, out)
		}
		unlocked, err := cgoGetStatus(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if unlocked {
			t.Fatal("expected unlocked=false for locked keychain")
		}
	})

	t.Run("nonexistent path returns error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nonexistent.keychain-db")
		if _, err := cgoGetStatus(path); err == nil {
			t.Fatal("expected error for nonexistent keychain, got nil")
		}
	})
}

func TestCgoCountItems(t *testing.T) {
	t.Run("empty keychain returns 0", func(t *testing.T) {
		path := newTempKeychain(t)
		n, err := cgoCountItems(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 0 {
			t.Fatalf("got %d, want 0", n)
		}
	})

	t.Run("count increases with each distinct add", func(t *testing.T) {
		path := newTempKeychain(t)
		refs := []string{"op://V/A/f", "op://V/B/f", "op://V/C/f"}

		for i, ref := range refs {
			if err := cgoAdd(path, ref, "acct", keychainKind, ref, []byte(`"v"`)); err != nil {
				t.Fatalf("cgoAdd %d: %v", i, err)
			}
			n, err := cgoCountItems(path)
			if err != nil {
				t.Fatalf("cgoCountItems after add %d: %v", i, err)
			}
			if n != i+1 {
				t.Fatalf("after %d adds: got %d, want %d", i+1, n, i+1)
			}
		}
	})

	t.Run("upsert does not increase count", func(t *testing.T) {
		path := newTempKeychain(t)
		ref := "op://V/Item/f"

		if err := cgoAdd(path, ref, "acct", keychainKind, ref, []byte(`"v1"`)); err != nil {
			t.Fatalf("first add: %v", err)
		}
		if err := cgoAdd(path, ref, "acct", keychainKind, ref, []byte(`"v2"`)); err != nil {
			t.Fatalf("second add (upsert): %v", err)
		}

		n, err := cgoCountItems(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 1 {
			t.Fatalf("got %d after upsert, want 1", n)
		}
	})

	t.Run("nonexistent path returns 0", func(t *testing.T) {
		// SecKeychainOpen is lazy (always noErr), so SecItemCopyMatching returns
		// errSecItemNotFound which kcCountItems maps to 0 rather than an error.
		path := filepath.Join(t.TempDir(), "nonexistent.keychain-db")
		n, err := cgoCountItems(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 0 {
			t.Fatalf("got %d, want 0 for nonexistent keychain", n)
		}
	})

	t.Run("non-cache description is not counted", func(t *testing.T) {
		path := newTempKeychain(t)
		// kcCountItems filters by kSecAttrDescription="1Password Cache" only.
		if err := cgoAdd(path, "op://V/Item/f", "acct", "Other", "op://V/Item/f", []byte(`"v"`)); err != nil {
			t.Fatalf("cgoAdd: %v", err)
		}
		n, err := cgoCountItems(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 0 {
			t.Fatalf("got %d, want 0: non-cache items should not be counted", n)
		}
	})
}

func TestCgoGetSettings(t *testing.T) {
	t.Run("default settings after create", func(t *testing.T) {
		path := newTempKeychain(t)
		s, err := cgoGetSettings(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// macOS defaults for a newly created keychain: lockOnSleep=true, lockInterval=300 (5 min).
		if !s.LockOnSleep {
			t.Error("expected LockOnSleep=true for new keychain (macOS default)")
		}
		if s.LockInterval != 300 {
			t.Errorf("got LockInterval=%d, want 300 for new keychain (macOS default)", s.LockInterval)
		}
	})

	t.Run("nonexistent path returns error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nonexistent.keychain-db")
		if _, err := cgoGetSettings(path); err == nil {
			t.Fatal("expected error for nonexistent keychain, got nil")
		}
	})
}
