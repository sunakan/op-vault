//go:build darwin

package keychain

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

		if out, err := exec.Command("security", "lock-keychain", path).CombinedOutput(); err != nil { //nolint:gosec // path is a t.TempDir() value, not user input; SecKeychainLock CGO wrapper is not implemented
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
		if out, err := exec.Command("security", "lock-keychain", path).CombinedOutput(); err != nil { //nolint:gosec // path is a t.TempDir() value, not user input; SecKeychainLock CGO wrapper is not implemented
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

func TestCgoClearItems(t *testing.T) {
	t.Run("empty keychain returns no error", func(t *testing.T) {
		path := newTempKeychain(t)
		if err := cgoClearItems(path); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("clears all items", func(t *testing.T) {
		path := newTempKeychain(t)
		refs := []string{"op://V/A/f", "op://V/B/f", "op://V/C/f"}
		for _, ref := range refs {
			if err := cgoAdd(path, ref, "acct", keychainKind, ref, []byte(`"v"`)); err != nil {
				t.Fatalf("cgoAdd %s: %v", ref, err)
			}
		}
		if err := cgoClearItems(path); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		n, err := cgoCountItems(path)
		if err != nil {
			t.Fatalf("cgoCountItems: %v", err)
		}
		if n != 0 {
			t.Fatalf("got %d items after clear, want 0", n)
		}
	})

	t.Run("keychain file still exists after clear", func(t *testing.T) {
		path := newTempKeychain(t)
		if err := cgoAdd(path, "op://V/Item/f", "acct", keychainKind, "op://V/Item/f", []byte(`"v"`)); err != nil {
			t.Fatalf("cgoAdd: %v", err)
		}
		if err := cgoClearItems(path); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("keychain file missing after clear: %v", err)
		}
	})

	t.Run("nonexistent path returns no error", func(t *testing.T) {
		// SecKeychainOpen is lazy (always noErr), so SecItemCopyMatching returns
		// errSecItemNotFound which kcClearItems maps to success rather than an error.
		// The not-found check is done at the keychain.Clear level via os.Stat.
		path := filepath.Join(t.TempDir(), "nonexistent.keychain-db")
		if err := cgoClearItems(path); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("non-cache items are not cleared", func(t *testing.T) {
		path := newTempKeychain(t)
		// Add one cache item and one non-cache item.
		if err := cgoAdd(path, "op://V/Cache/f", "acct", keychainKind, "op://V/Cache/f", []byte(`"v"`)); err != nil {
			t.Fatalf("cgoAdd cache item: %v", err)
		}
		if err := cgoAdd(path, "op://V/Other/f", "acct", "Other", "op://V/Other/f", []byte(`"v"`)); err != nil {
			t.Fatalf("cgoAdd non-cache item: %v", err)
		}
		if err := cgoClearItems(path); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// cgoCountItems only counts cache items, so result should be 0.
		n, err := cgoCountItems(path)
		if err != nil {
			t.Fatalf("cgoCountItems: %v", err)
		}
		if n != 0 {
			t.Fatalf("got %d cache items after clear, want 0", n)
		}
		// The non-cache item should still be retrievable.
		_, found, err := cgoGet(path, "op://V/Other/f", "acct")
		if err != nil {
			t.Fatalf("cgoGet non-cache item: %v", err)
		}
		if !found {
			t.Fatal("expected non-cache item to survive clear")
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

	t.Run("custom lock interval is read back correctly", func(t *testing.T) {
		path := newTempKeychain(t)
		if out, err := exec.Command("security", "set-keychain-settings", "-t", "600", path).CombinedOutput(); err != nil { //nolint:gosec
			t.Fatalf("set-keychain-settings: %v: %s", err, out)
		}
		s, err := cgoGetSettings(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.LockInterval != 600 {
			t.Errorf("got LockInterval=%d, want 600", s.LockInterval)
		}
	})

	t.Run("lock-on-sleep disabled", func(t *testing.T) {
		path := newTempKeychain(t)
		// no -l flag disables lock-on-sleep
		if out, err := exec.Command("security", "set-keychain-settings", path).CombinedOutput(); err != nil { //nolint:gosec
			t.Fatalf("set-keychain-settings: %v: %s", err, out)
		}
		s, err := cgoGetSettings(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.LockOnSleep {
			t.Error("expected LockOnSleep=false after set-keychain-settings without -l")
		}
	})

	t.Run("lock-on-sleep enabled", func(t *testing.T) {
		path := newTempKeychain(t)
		// -l enables lock-on-sleep
		if out, err := exec.Command("security", "set-keychain-settings", "-l", path).CombinedOutput(); err != nil { //nolint:gosec
			t.Fatalf("set-keychain-settings -l: %v: %s", err, out)
		}
		s, err := cgoGetSettings(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !s.LockOnSleep {
			t.Error("expected LockOnSleep=true after set-keychain-settings -l")
		}
	})
}

func TestCgoList(t *testing.T) {
	t.Run("empty keychain returns nil", func(t *testing.T) {
		path := newTempKeychain(t)
		entries, err := cgoList(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("got %d entries, want 0", len(entries))
		}
	})

	t.Run("returns ref and account for added item", func(t *testing.T) {
		path := newTempKeychain(t)
		ref, account := "op://V/Item/f", "myaccount"
		if err := cgoAdd(path, ref, account, keychainKind, ref, []byte(`"v"`)); err != nil {
			t.Fatalf("cgoAdd: %v", err)
		}
		entries, err := cgoList(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("got %d entries, want 1", len(entries))
		}
		if entries[0].Ref != ref {
			t.Errorf("Ref: got %q, want %q", entries[0].Ref, ref)
		}
		if entries[0].Account != account {
			t.Errorf("Account: got %q, want %q", entries[0].Account, account)
		}
	})

	t.Run("UpdatedAt matches YYYY-MM-DD HH:MM:SS", func(t *testing.T) {
		path := newTempKeychain(t)
		if err := cgoAdd(path, "op://V/Item/f", "acct", keychainKind, "op://V/Item/f", []byte(`"v"`)); err != nil {
			t.Fatalf("cgoAdd: %v", err)
		}
		entries, err := cgoList(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("got %d entries, want 1", len(entries))
		}
		matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`, entries[0].UpdatedAt)
		if !matched {
			t.Errorf("UpdatedAt %q does not match YYYY-MM-DD HH:MM:SS", entries[0].UpdatedAt)
		}
	})

	t.Run("multiple items count matches", func(t *testing.T) {
		path := newTempKeychain(t)
		refs := []string{"op://V/A/f", "op://V/B/f", "op://V/C/f"}
		for _, ref := range refs {
			if err := cgoAdd(path, ref, "acct", keychainKind, ref, []byte(`"v"`)); err != nil {
				t.Fatalf("cgoAdd %s: %v", ref, err)
			}
		}
		entries, err := cgoList(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != len(refs) {
			t.Fatalf("got %d entries, want %d", len(entries), len(refs))
		}
	})

	t.Run("accounts preserved across multiple items", func(t *testing.T) {
		path := newTempKeychain(t)
		if err := cgoAdd(path, "op://V/A/f", "account-a", keychainKind, "op://V/A/f", []byte(`"v"`)); err != nil {
			t.Fatalf("cgoAdd A: %v", err)
		}
		if err := cgoAdd(path, "op://V/B/f", "account-b", keychainKind, "op://V/B/f", []byte(`"v"`)); err != nil {
			t.Fatalf("cgoAdd B: %v", err)
		}
		entries, err := cgoList(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("got %d entries, want 2", len(entries))
		}
		accountByRef := make(map[string]string, len(entries))
		for _, e := range entries {
			accountByRef[e.Ref] = e.Account
		}
		if accountByRef["op://V/A/f"] != "account-a" {
			t.Errorf("A: got account %q, want account-a", accountByRef["op://V/A/f"])
		}
		if accountByRef["op://V/B/f"] != "account-b" {
			t.Errorf("B: got account %q, want account-b", accountByRef["op://V/B/f"])
		}
	})

	t.Run("non-cache items not listed", func(t *testing.T) {
		path := newTempKeychain(t)
		if err := cgoAdd(path, "op://V/Cache/f", "acct", keychainKind, "op://V/Cache/f", []byte(`"v"`)); err != nil {
			t.Fatalf("cgoAdd cache: %v", err)
		}
		if err := cgoAdd(path, "op://V/Other/f", "acct", "Other", "op://V/Other/f", []byte(`"v"`)); err != nil {
			t.Fatalf("cgoAdd non-cache: %v", err)
		}
		entries, err := cgoList(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("got %d entries, want 1 (non-cache item must be excluded)", len(entries))
		}
		if entries[0].Ref != "op://V/Cache/f" {
			t.Errorf("expected cache item, got %q", entries[0].Ref)
		}
	})

	t.Run("nonexistent keychain returns nil", func(t *testing.T) {
		// SecKeychainOpen is lazy (always noErr), so SecItemCopyMatching returns
		// errSecItemNotFound which kcList maps to an empty result.
		path := filepath.Join(t.TempDir(), "nonexistent.keychain-db")
		entries, err := cgoList(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("got %d entries, want 0", len(entries))
		}
	})
}

func TestCgoDeleteItem(t *testing.T) {
	t.Run("delete existing item", func(t *testing.T) {
		path := newTempKeychain(t)
		ref, account := "op://V/Item/f", "acct"
		if err := cgoAdd(path, ref, account, keychainKind, ref, []byte(`"v"`)); err != nil {
			t.Fatalf("cgoAdd: %v", err)
		}
		if err := cgoDeleteItem(path, ref, account); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, found, err := cgoGet(path, ref, account)
		if err != nil {
			t.Fatalf("cgoGet after delete: %v", err)
		}
		if found {
			t.Fatal("expected item to be gone after delete")
		}
	})

	t.Run("count decreases after delete", func(t *testing.T) {
		path := newTempKeychain(t)
		refs := []string{"op://V/A/f", "op://V/B/f", "op://V/C/f"}
		for _, ref := range refs {
			if err := cgoAdd(path, ref, "acct", keychainKind, ref, []byte(`"v"`)); err != nil {
				t.Fatalf("cgoAdd %s: %v", ref, err)
			}
		}
		if err := cgoDeleteItem(path, "op://V/B/f", "acct"); err != nil {
			t.Fatalf("cgoDeleteItem: %v", err)
		}
		n, err := cgoCountItems(path)
		if err != nil {
			t.Fatalf("cgoCountItems: %v", err)
		}
		if n != 2 {
			t.Fatalf("got %d items, want 2 after deleting one of three", n)
		}
	})

	t.Run("delete nonexistent item returns error", func(t *testing.T) {
		path := newTempKeychain(t)
		if err := cgoDeleteItem(path, "op://V/Missing/f", "acct"); err == nil {
			t.Fatal("expected error for nonexistent item, got nil")
		}
	})

	t.Run("wrong account does not delete item", func(t *testing.T) {
		path := newTempKeychain(t)
		ref := "op://V/Item/f"
		if err := cgoAdd(path, ref, "account-a", keychainKind, ref, []byte(`"v"`)); err != nil {
			t.Fatalf("cgoAdd: %v", err)
		}
		if err := cgoDeleteItem(path, ref, "account-b"); err == nil {
			t.Fatal("expected error when deleting with wrong account, got nil")
		}
		_, found, err := cgoGet(path, ref, "account-a")
		if err != nil {
			t.Fatalf("cgoGet: %v", err)
		}
		if !found {
			t.Fatal("expected original item to survive delete with wrong account")
		}
	})

	t.Run("non-cache item is not deleted", func(t *testing.T) {
		// kcDeleteItem scopes the query with kSecAttrDescription="1Password Cache".
		// An item stored with a different description must not be deleted.
		path := newTempKeychain(t)
		ref, account := "op://V/Item/f", "acct"
		if err := cgoAdd(path, ref, account, "Other", ref, []byte(`"v"`)); err != nil {
			t.Fatalf("cgoAdd non-cache: %v", err)
		}
		if err := cgoDeleteItem(path, ref, account); err == nil {
			t.Fatal("expected error when deleting non-cache item, got nil")
		}
		_, found, err := cgoGet(path, ref, account)
		if err != nil {
			t.Fatalf("cgoGet: %v", err)
		}
		if !found {
			t.Fatal("expected non-cache item to survive cgoDeleteItem")
		}
	})
}
