//go:build integration

package keychain_test

import (
	"errors"
	"os"
	"testing"

	"github.com/sunakan/op-keychain/internal/keychain"
)

// 実運用の keychain を汚さないよう専用名を使う
const testKeychainName = "op-keychain-integration-test"

func newTestKeychain(t *testing.T) keychain.Keychain {
	t.Helper()
	t.Setenv("OP_KEYCHAIN_NAME", testKeychainName)
	kc := keychain.NewExecKeychain()

	// 前回テストの残骸があれば削除
	if exists, _ := kc.Exists(); exists {
		if err := kc.Delete(); err != nil {
			t.Fatalf("cleanup existing keychain: %v", err)
		}
	}

	t.Cleanup(func() {
		if exists, _ := kc.Exists(); exists {
			_ = kc.Delete()
		}
		// keychain list から除外するため環境変数を元に戻す
		os.Unsetenv("OP_KEYCHAIN_NAME")
	})

	return kc
}

func TestIntegration_CreateAndExists(t *testing.T) {
	kc := newTestKeychain(t)

	exists, err := kc.Exists()
	if err != nil || exists {
		t.Fatalf("want false before create, got exists=%v err=%v", exists, err)
	}

	if err := kc.Create("", 3600e9); err != nil {
		t.Fatalf("Create: %v", err)
	}

	exists, err = kc.Exists()
	if err != nil || !exists {
		t.Fatalf("want true after create, got exists=%v err=%v", exists, err)
	}
}

func TestIntegration_SetAndGet(t *testing.T) {
	kc := newTestKeychain(t)
	if err := kc.Create("", 3600e9); err != nil {
		t.Fatalf("Create: %v", err)
	}

	svc := keychain.Service("op://vault/item/field")
	const want = `{"ref":"op://vault/item/field","name":"item","value":"s3cr3t","account":""}`

	if err := kc.Set(svc, want); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := kc.Get(svc)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != want {
		t.Errorf("Get = %q, want %q", got, want)
	}
}

func TestIntegration_GetNotFound(t *testing.T) {
	kc := newTestKeychain(t)
	if err := kc.Create("", 3600e9); err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err := kc.Get(keychain.Service("op://vault/item/nonexistent"))
	if !errors.Is(err, keychain.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestIntegration_Remove(t *testing.T) {
	kc := newTestKeychain(t)
	if err := kc.Create("", 3600e9); err != nil {
		t.Fatalf("Create: %v", err)
	}

	svc := keychain.Service("op://vault/item/field")
	if err := kc.Set(svc, `{"ref":"op://vault/item/field","name":"","value":"v","account":""}`); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if err := kc.Remove(svc); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	_, err := kc.Get(svc)
	if !errors.Is(err, keychain.ErrNotFound) {
		t.Errorf("want ErrNotFound after remove, got %v", err)
	}
}

func TestIntegration_RemoveNotFound(t *testing.T) {
	kc := newTestKeychain(t)
	if err := kc.Create("", 3600e9); err != nil {
		t.Fatalf("Create: %v", err)
	}

	err := kc.Remove(keychain.Service("op://vault/item/nonexistent"))
	if !errors.Is(err, keychain.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestIntegration_ListServices(t *testing.T) {
	kc := newTestKeychain(t)
	if err := kc.Create("", 3600e9); err != nil {
		t.Fatalf("Create: %v", err)
	}

	refs := []string{
		"op://vault/item1/field",
		"op://vault/item2/field",
	}
	for _, ref := range refs {
		if err := kc.Set(keychain.Service(ref), `{"ref":"`+ref+`","name":"","value":"v","account":""}`); err != nil {
			t.Fatalf("Set %s: %v", ref, err)
		}
	}

	svcs, err := kc.ListServices()
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(svcs) != len(refs) {
		t.Errorf("ListServices len = %d, want %d", len(svcs), len(refs))
	}
}

func TestIntegration_Delete(t *testing.T) {
	kc := newTestKeychain(t)
	if err := kc.Create("", 3600e9); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := kc.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	exists, err := kc.Exists()
	if err != nil || exists {
		t.Errorf("want false after delete, got exists=%v err=%v", exists, err)
	}
}

func TestIntegration_IdleTimeout(t *testing.T) {
	kc := newTestKeychain(t)
	if err := kc.Create("", 3600e9); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := kc.SetIdleTimeout(1800); err != nil {
		t.Fatalf("SetIdleTimeout: %v", err)
	}

	got, err := kc.GetIdleTimeout()
	if err != nil {
		t.Fatalf("GetIdleTimeout: %v", err)
	}
	if got != 1800 {
		t.Errorf("GetIdleTimeout = %d, want 1800", got)
	}
}
