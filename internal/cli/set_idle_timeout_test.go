//go:build darwin

package cli

import (
	"strings"
	"testing"

	kfake "github.com/sunakan/op-keychain/internal/keychain/fake"
)

func TestSetIdleTimeoutPositive(t *testing.T) {
	kc := kfake.New()

	out, _, code, err := runCapture(func() error {
		return (&SetIdleTimeoutCmd{Seconds: 300, KC: kc}).Run()
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "300s") {
		t.Errorf("stdout = %q, want '300s'", out)
	}
	if kc.IdleTimeout != 300 {
		t.Errorf("IdleTimeout = %d, want 300", kc.IdleTimeout)
	}
}

func TestSetIdleTimeoutZero(t *testing.T) {
	kc := kfake.New()

	_, errOut, code, _ := runCapture(func() error {
		return (&SetIdleTimeoutCmd{Seconds: 0, KC: kc}).Run()
	})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "positive integer") {
		t.Errorf("stderr = %q, want 'positive integer'", errOut)
	}
}

func TestSetIdleTimeoutNegative(t *testing.T) {
	kc := kfake.New()

	_, _, code, _ := runCapture(func() error {
		return (&SetIdleTimeoutCmd{Seconds: -1, KC: kc}).Run()
	})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestSetIdleTimeoutKeychainNotExists(t *testing.T) {
	kc := kfake.New()
	kc.ExistsVal = false

	_, _, code, err := runCapture(func() error {
		return (&SetIdleTimeoutCmd{Seconds: 300, KC: kc}).Run()
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (error returned)", code)
	}
	if err == nil {
		t.Fatal("expected error when keychain not exists")
	}
}
