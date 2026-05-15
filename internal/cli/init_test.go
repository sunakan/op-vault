//go:build darwin

package cli

import (
	"errors"
	"strings"
	"testing"

	kfake "github.com/sunakan/op-keychain/internal/keychain/fake"
)

func TestInitAlreadyInitialized(t *testing.T) {
	kc := kfake.New()
	kc.ExistsVal = true

	out, _, code, err := runCapture(func() error {
		return (&InitCmd{KC: kc}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "already initialized") {
		t.Errorf("stdout = %q, want 'already initialized'", out)
	}
}

func TestAutoCreate(t *testing.T) {
	kc := kfake.New()
	kc.ExistsVal = false

	if err := AutoCreate(kc); err != nil {
		t.Fatalf("AutoCreate error: %v", err)
	}
	if !kc.ExistsVal {
		t.Error("keychain not created")
	}
}

func TestIdleTimeoutDefault(t *testing.T) {
	t.Setenv("OP_KEYCHAIN_IDLE_TIMEOUT", "")
	d := idleTimeout()
	if d.Seconds() != 3600 {
		t.Errorf("idleTimeout = %v, want 3600s", d)
	}
}

func TestIdleTimeoutCustom(t *testing.T) {
	t.Setenv("OP_KEYCHAIN_IDLE_TIMEOUT", "600")
	d := idleTimeout()
	if d.Seconds() != 600 {
		t.Errorf("idleTimeout = %v, want 600s", d)
	}
}

func TestIdleTimeoutInvalid(t *testing.T) {
	t.Setenv("OP_KEYCHAIN_IDLE_TIMEOUT", "notanumber")
	d := idleTimeout()
	if d.Seconds() != 3600 {
		t.Errorf("idleTimeout = %v, want 3600s (fallback)", d)
	}
}

func TestIdleTimeoutNonPositive(t *testing.T) {
	t.Setenv("OP_KEYCHAIN_IDLE_TIMEOUT", "0")
	d := idleTimeout()
	if d.Seconds() != 3600 {
		t.Errorf("idleTimeout = %v, want 3600s (fallback for 0)", d)
	}
}

func TestAutoCreateCreateFails(t *testing.T) {
	kc := kfake.New()
	kc.ExistsVal = false
	kc.CreateErr = errors.New("disk full")

	if err := AutoCreate(kc); err == nil {
		t.Fatal("expected error when Create fails")
	}
}
