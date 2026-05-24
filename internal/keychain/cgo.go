//go:build darwin

package keychain

/*
#cgo LDFLAGS: -framework CoreFoundation -framework Security
#pragma clang diagnostic ignored "-Wdeprecated-declarations"

#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>
#include <stdlib.h>

// kcCreate creates a new keychain file at path, protected by password.
// SecKeychainCreate is deprecated since macOS 12 — there is no non-deprecated
// API to create a custom keychain file for a CLI tool.
static OSStatus kcCreate(const char *path, const char *password, int pwLen) {
	SecKeychainRef ref = NULL;
	OSStatus err = SecKeychainCreate(path, (UInt32)pwLen, password,
	                                 (Boolean)0, NULL, &ref);
	if (ref) CFRelease(ref);
	return err;
}

// kcOpen opens an existing keychain at path and returns its SecKeychainRef.
// Caller must CFRelease the returned ref.
// SecKeychainOpen is deprecated since macOS 12 — needed to target a specific
// keychain file for SecItemAdd/SecItemCopyMatching via kSecUseKeychain /
// kSecMatchSearchList.
static OSStatus kcOpen(const char *path, SecKeychainRef *out) {
	return SecKeychainOpen(path, out);
}

// kcAdd adds a generic password to the keychain referenced by kref.
// SecItemAdd itself is not deprecated; only obtaining kref requires deprecated API.
// kSecAttrAccess with NULL trusted list — any app can read without a permission dialog.
// Without this, macOS binds the ACL to the creating app's code signature and prompts
// every other caller (including `security` CLI used in E2E and re-built binaries).
static OSStatus kcAdd(SecKeychainRef kref,
                      const char *service, const char *account,
                      const char *description,
                      const void *data, int dataLen) {
	CFStringRef svc  = CFStringCreateWithCString(NULL, service,     kCFStringEncodingUTF8);
	CFStringRef acc  = CFStringCreateWithCString(NULL, account,     kCFStringEncodingUTF8);
	CFStringRef desc = CFStringCreateWithCString(NULL, description, kCFStringEncodingUTF8);
	CFDataRef   dat  = CFDataCreate(NULL, (const UInt8 *)data, (CFIndex)dataLen);

	SecAccessRef access = NULL;
	SecAccessCreate(svc, NULL, &access);

	const void *keys[] = {
		kSecClass, kSecAttrService, kSecAttrAccount,
		kSecAttrDescription, kSecValueData, kSecUseKeychain, kSecAttrAccess,
	};
	const void *vals[] = {
		kSecClassGenericPassword, svc, acc, desc, dat, kref, access,
	};
	CFDictionaryRef attrs = CFDictionaryCreate(NULL, keys, vals, 7,
	    &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);

	OSStatus err = SecItemAdd(attrs, NULL);

	CFRelease(attrs);
	if (access) CFRelease(access);
	CFRelease(dat);
	CFRelease(desc);
	CFRelease(acc);
	CFRelease(svc);
	return err;
}


*/
import "C" //nolint:gocritic // CGO requires import "C" as its own statement immediately after the C comment block
import (
	"fmt"
	"unsafe" //nolint:gocritic
)

func cgoCreate(path, password string) error {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	pw := C.CString(password)
	defer C.free(unsafe.Pointer(pw))
	if code := C.kcCreate(p, pw, C.int(len(password))); code != 0 {
		return fmt.Errorf("SecKeychainCreate: OSStatus %d", int(code))
	}
	return nil
}

func cgoAdd(path, service, account, description string, data []byte) error {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))

	var kref C.SecKeychainRef
	if code := C.kcOpen(p, &kref); code != 0 { //nolint:gocritic // false positive: gocritic misidentifies CGO out-param pattern as dupSubExpr
		return fmt.Errorf("SecKeychainOpen: OSStatus %d", int(code))
	}
	defer C.CFRelease(C.CFTypeRef(kref))

	svc := C.CString(service)
	defer C.free(unsafe.Pointer(svc))
	acc := C.CString(account)
	defer C.free(unsafe.Pointer(acc))
	desc := C.CString(description)
	defer C.free(unsafe.Pointer(desc))

	var dataPtr unsafe.Pointer
	if len(data) > 0 {
		dataPtr = unsafe.Pointer(&data[0])
	}
	if code := C.kcAdd(kref, svc, acc, desc, dataPtr, C.int(len(data))); code != 0 {
		return fmt.Errorf("SecItemAdd: OSStatus %d", int(code))
	}
	return nil
}
