//go:build darwin

package keychain

import "fmt"

// osStatusString converts a macOS Security framework OSStatus code to a
// human-readable string. Covers codes returned by SecItem* and SecKeychain*
// APIs used in this package. Unknown codes fall back to "OSStatus N".
func osStatusString(code int) string {
	switch code {
	case -4:
		return "unimplemented"
	case -50:
		return "invalid parameter"
	case -108:
		return "memory allocation failed"
	case -128:
		return "user canceled"
	case -25243:
		return "no access for item"
	case -25291:
		return "no keychain available"
	case -25292:
		return "keychain is read-only"
	case -25293:
		return "auth failed"
	case -25294:
		return "keychain not found"
	case -25295:
		return "invalid keychain"
	case -25296:
		return "duplicate keychain name"
	case -25299:
		return "duplicate item"
	case -25300:
		return "item not found"
	case -25303:
		return "no such attribute"
	case -25308:
		return "interaction not allowed"
	case -25315:
		return "interaction required"
	case -25320:
		return "UI unavailable in dark wake"
	case -26275:
		return "decode failed"
	default:
		return fmt.Sprintf("OSStatus %d", code)
	}
}
