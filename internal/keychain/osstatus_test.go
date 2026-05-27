//go:build darwin

package keychain

import (
	"fmt"
	"testing"
)

func TestOsStatusString(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		// 既知コード
		{-4, "unimplemented"},
		{-50, "invalid parameter"},
		{-108, "memory allocation failed"},
		{-128, "user canceled"},
		{-25243, "no access for item"},
		{-25291, "no keychain available"},
		{-25292, "keychain is read-only"},
		{-25293, "auth failed"},
		{-25294, "keychain not found"},
		{-25295, "invalid keychain"},
		{-25296, "duplicate keychain name"},
		{-25299, "duplicate item"},
		{-25300, "item not found"},
		{-25303, "no such attribute"},
		{-25308, "interaction not allowed"},
		{-25315, "interaction required"},
		{-25320, "UI unavailable in dark wake"},
		{-26275, "decode failed"},
		// 未知コード: フォールバック
		{0, "OSStatus 0"},
		{1, "OSStatus 1"},
		{-99999, "OSStatus -99999"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("code_%d", tt.code), func(t *testing.T) {
			got := osStatusString(tt.code)
			if got != tt.want {
				t.Errorf("osStatusString(%d) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}
