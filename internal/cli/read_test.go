//go:build darwin

package cli

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/sunakan/op-keychain/internal/keychain"
	kfake "github.com/sunakan/op-keychain/internal/keychain/fake"
	opfake "github.com/sunakan/op-keychain/internal/op/fake"
)

func storeEntry(kc *kfake.Keychain, ref, name, value, account string) {
	e := keychain.Entry{Ref: ref, Name: name, Value: value, Account: account}
	data, _ := json.Marshal(e)
	kc.Entries[keychain.Service(ref)] = string(data)
}

func TestReadCacheHit(t *testing.T) {
	kc := kfake.New()
	ref := "op://vault/item/field"
	storeEntry(kc, ref, "Item", "secretvalue", "myaccount")

	out, _, code, err := runCapture(func() error {
		return (&ReadCmd{Ref: ref, KC: kc}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if out != "secretvalue" {
		t.Errorf("stdout = %q, want %q", out, "secretvalue")
	}
}

func TestReadCacheMiss(t *testing.T) {
	kc := kfake.New()
	ref := "op://vault/item/field"
	op := opfake.New()
	op.Secrets[ref] = "freshvalue"
	op.Titles[ref] = "Item"

	out, _, code, err := runCapture(func() error {
		return (&ReadCmd{Ref: ref, KC: kc, OP: op}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if out != "freshvalue" {
		t.Errorf("stdout = %q, want %q", out, "freshvalue")
	}

	svc := keychain.Service(ref)
	if _, ok := kc.Entries[svc]; !ok {
		t.Error("entry not stored in keychain after cache miss")
	}
}

func TestReadSDKFailure(t *testing.T) {
	kc := kfake.New()
	ref := "op://vault/item/field"
	op := opfake.New()
	op.ResolveErr = errors.New("auth failed")

	_, errOut, code, _ := runCapture(func() error {
		return (&ReadCmd{Ref: ref, KC: kc, OP: op}).Run()
	})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if errOut == "" {
		t.Error("expected error message on stderr")
	}
}

func TestReadInvalidRef(t *testing.T) {
	kc := kfake.New()
	_, _, code, _ := runCapture(func() error {
		return (&ReadCmd{Ref: "not-a-ref", KC: kc}).Run()
	})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestReadAutoCreate(t *testing.T) {
	kc := kfake.New()
	kc.ExistsVal = false
	ref := "op://vault/item/field"
	op := opfake.New()
	op.Secrets[ref] = "autovalue"
	op.Titles[ref] = "Item"

	out, _, code, err := runCapture(func() error {
		return (&ReadCmd{Ref: ref, KC: kc, OP: op}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if out != "autovalue" {
		t.Errorf("stdout = %q, want %q", out, "autovalue")
	}
	if !kc.ExistsVal {
		t.Error("keychain not created by AutoCreate")
	}
}

func TestReadNonASCIIRoundtrip(t *testing.T) {
	kc := kfake.New()
	ref := "op://vault/item/field"
	const value = "こんにちは"
	storeEntry(kc, ref, "", value, "")

	out, _, code, err := runCapture(func() error {
		return (&ReadCmd{Ref: ref, KC: kc}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if out != value {
		t.Errorf("stdout = %q, want %q", out, value)
	}
}

func TestReadNewlineRoundtrip(t *testing.T) {
	kc := kfake.New()
	ref := "op://vault/item/field"
	const value = "line1\nline2"
	storeEntry(kc, ref, "", value, "")

	out, _, code, err := runCapture(func() error {
		return (&ReadCmd{Ref: ref, KC: kc}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if out != value {
		t.Errorf("stdout = %q, want %q", out, value)
	}
}

func TestReadCacheHitInvalidJSON(t *testing.T) {
	kc := kfake.New()
	ref := "op://vault/item/field"
	// invalid JSON → fall through to cache miss path
	kc.Entries[keychain.Service(ref)] = "not-json"
	op := opfake.New()
	op.Secrets[ref] = "resolved"
	op.Titles[ref] = "Item"

	out, _, code, err := runCapture(func() error {
		return (&ReadCmd{Ref: ref, KC: kc, OP: op}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if out != "resolved" {
		t.Errorf("stdout = %q, want 'resolved'", out)
	}
}

func TestReadLockedKeychainSetRetry(t *testing.T) {
	kc := kfake.New()
	kc.Locked = true
	ref := "op://vault/item/field"
	op := opfake.New()
	op.Secrets[ref] = "val"
	op.Titles[ref] = "Item"

	out, _, code, err := runCapture(func() error {
		return (&ReadCmd{Ref: ref, KC: kc, OP: op}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if out != "val" {
		t.Errorf("stdout = %q, want 'val'", out)
	}
}

func TestReadAccountFieldStored(t *testing.T) {
	kc := kfake.New()
	ref := "op://vault/item/field"
	op := opfake.New()
	op.Secrets[ref] = "val"

	_, _, _, _ = runCapture(func() error {
		return (&ReadCmd{Ref: ref, KC: kc, OP: op, Account: ""}).Run()
	})

	svc := keychain.Service(ref)
	raw, ok := kc.Entries[svc]
	if !ok {
		t.Fatal("entry not stored")
	}
	var e keychain.Entry
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if e.Account != "" {
		t.Errorf("account = %q, want empty", e.Account)
	}
}
