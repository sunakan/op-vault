//go:build darwin

package keychain

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <Security/Security.h>
#include <stdlib.h>

// Returns 0=unlocked, 1=locked, -1=error.
// SecKeychainGetStatus reads keychain metadata without unlocking or showing dialogs.
// SecKeychainOpen/SecKeychainGetStatus are deprecated since macOS 10.10 but remain
// the only way to query lock status without triggering a SecurityAgent dialog.
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wdeprecated-declarations"
static int keychainIsLocked(const char *path) {
    // Suppress SecurityAgent dialog; locked keychain returns errSecInteractionNotAllowed.
    SecKeychainSetUserInteractionAllowed(FALSE);
    SecKeychainRef kc = NULL;
    OSStatus openErr = SecKeychainOpen(path, &kc);
    if (openErr != errSecSuccess || kc == NULL) {
        SecKeychainSetUserInteractionAllowed(TRUE);
        return (openErr == errSecInteractionNotAllowed || openErr == errSecAuthFailed) ? 1 : -1;
    }
    SecKeychainStatus status = 0;
    OSStatus err = SecKeychainGetStatus(kc, &status);
    CFRelease(kc);
    SecKeychainSetUserInteractionAllowed(TRUE);
    if (err != errSecSuccess) {
        return (err == errSecInteractionNotAllowed || err == errSecAuthFailed) ? 1 : -1;
    }
    return (status & kSecUnlockStateStatus) ? 0 : 1;
}
#pragma GCC diagnostic pop
*/
import "C"
import (
	"fmt"
	"unsafe"
)

func (k *ExecKeychain) IsLocked() (bool, error) {
	cpath := C.CString(k.path)
	defer C.free(unsafe.Pointer(cpath))
	switch C.keychainIsLocked(cpath) {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("failed to get keychain lock status: %s", k.path)
	}
}
