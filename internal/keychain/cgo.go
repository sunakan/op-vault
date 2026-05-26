//go:build darwin

package keychain

/*
#cgo LDFLAGS: -framework CoreFoundation -framework Security
#pragma clang diagnostic ignored "-Wdeprecated-declarations"

#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>
#include <stdlib.h>

// SecKeychainCopySettings: present in Security.framework binary (verified via .tbd)
// but absent from public SDK headers — forward-declare to call it directly.
extern OSStatus SecKeychainCopySettings(SecKeychainRef keychain, SecKeychainSettings *outSettings);

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

// kcGetStatus returns 1 if unlocked, 0 if locked, -1 on error.
static int kcGetStatus(const char *path) {
	SecKeychainRef ref = NULL;
	if (SecKeychainOpen(path, &ref) != noErr || ref == NULL) {
		return -1;
	}
	SecKeychainStatus status = 0;
	OSStatus err = SecKeychainGetStatus(ref, &status);
	CFRelease(ref);
	if (err != noErr) {
		return -1;
	}
	return (status & kSecUnlockStateStatus) ? 1 : 0;
}

// kcCountItems returns the number of "1Password Cache" generic password items
// in the keychain at path, or -1 on error.
static int kcCountItems(const char *path) {
	SecKeychainRef ref = NULL;
	if (SecKeychainOpen(path, &ref) != noErr || ref == NULL) {
		return -1;
	}
	CFStringRef desc = CFStringCreateWithCString(NULL, "1Password Cache", kCFStringEncodingUTF8);
	CFArrayRef searchList = CFArrayCreate(NULL, (const void **)&ref, 1, &kCFTypeArrayCallBacks);
	const void *keys[] = {
		kSecClass, kSecAttrDescription, kSecMatchSearchList, kSecMatchLimit, kSecReturnAttributes,
	};
	const void *vals[] = {
		kSecClassGenericPassword, desc, searchList, kSecMatchLimitAll, kCFBooleanTrue,
	};
	CFDictionaryRef query = CFDictionaryCreate(NULL, keys, vals, 5,
	    &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFTypeRef result = NULL;
	OSStatus err = SecItemCopyMatching(query, &result);
	CFRelease(query);
	CFRelease(searchList);
	CFRelease(desc);
	CFRelease(ref);
	int count = 0;
	if (err == noErr && result != NULL) {
		count = (int)CFArrayGetCount((CFArrayRef)result);
		CFRelease(result);
	} else if (err == errSecItemNotFound) {
		count = 0;
	} else {
		count = -1;
	}
	return count;
}

// kcGet retrieves password data for service/account from the keychain at path.
// Returns 0 (found, caller must free *outData), 1 (not found), or -1 (error).
static int kcGet(const char *path, const char *service, const char *account,
                 void **outData, int *outLen) {
	SecKeychainRef ref = NULL;
	if (SecKeychainOpen(path, &ref) != noErr || ref == NULL) {
		return -1;
	}
	CFStringRef svc = CFStringCreateWithCString(NULL, service, kCFStringEncodingUTF8);
	CFStringRef acc = CFStringCreateWithCString(NULL, account, kCFStringEncodingUTF8);
	CFArrayRef searchList = CFArrayCreate(NULL, (const void **)&ref, 1, &kCFTypeArrayCallBacks);
	const void *keys[] = {
		kSecClass, kSecAttrService, kSecAttrAccount,
		kSecMatchSearchList, kSecMatchLimit, kSecReturnData,
	};
	const void *vals[] = {
		kSecClassGenericPassword, svc, acc,
		searchList, kSecMatchLimitOne, kCFBooleanTrue,
	};
	CFDictionaryRef query = CFDictionaryCreate(NULL, keys, vals, 6,
	    &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFTypeRef result = NULL;
	OSStatus err = SecItemCopyMatching(query, &result);
	CFRelease(query);
	CFRelease(searchList);
	CFRelease(acc);
	CFRelease(svc);
	CFRelease(ref);
	if (err == errSecItemNotFound) {
		*outData = NULL; *outLen = 0;
		return 1;
	}
	if (err != noErr || result == NULL) {
		return -1;
	}
	CFDataRef dat = (CFDataRef)result;
	CFIndex len = CFDataGetLength(dat);
	void *buf = malloc((size_t)len);
	memcpy(buf, CFDataGetBytePtr(dat), (size_t)len);
	CFRelease(result);
	*outData = buf;
	*outLen = (int)len;
	return 0;
}

// kcGetSettings retrieves lock settings from the keychain at path.
// Returns 0 on success, -1 on error.
// SecKeychainCopySettings always sets useLockInterval=false on arm64 macOS 15+
// even when a timeout is stored; callers should rely on lockInterval directly.
static int kcGetSettings(const char *path,
                         int *lockOnSleep, unsigned int *lockInterval) {
	SecKeychainRef ref = NULL;
	if (SecKeychainOpen(path, &ref) != noErr || ref == NULL) {
		return -1;
	}
	SecKeychainSettings settings = { SEC_KEYCHAIN_SETTINGS_VERS1, (Boolean)0, (Boolean)0, 0 };
	OSStatus err = SecKeychainCopySettings(ref, &settings);
	CFRelease(ref);
	if (err != noErr) {
		return -1;
	}
	*lockOnSleep = settings.lockOnSleep ? 1 : 0;
	*lockInterval = (unsigned int)settings.lockInterval;
	return 0;
}

// kcAdd adds or updates a generic password in the keychain referenced by kref.
// On errSecDuplicateItem, falls back to SecItemUpdate to overwrite the value.
// kSecAttrAccess restricts reads to the current binary only — prevents other processes
// from silently reading cached 1Password secrets.
// NULL trusted list (any app can read) was the previous approach; replaced to limit
// silent access to cached secrets.
static OSStatus kcAdd(SecKeychainRef kref,
                      const char *service, const char *account,
                      const char *description,
                      const void *data, int dataLen) {
	CFStringRef svc  = CFStringCreateWithCString(NULL, service,     kCFStringEncodingUTF8);
	CFStringRef acc  = CFStringCreateWithCString(NULL, account,     kCFStringEncodingUTF8);
	CFStringRef desc = CFStringCreateWithCString(NULL, description, kCFStringEncodingUTF8);
	CFDataRef   dat  = CFDataCreate(NULL, (const UInt8 *)data, (CFIndex)dataLen);

	SecTrustedApplicationRef selfApp = NULL;
	SecTrustedApplicationCreateFromPath(NULL, &selfApp);
	CFArrayRef trustedList = CFArrayCreate(NULL, (const void **)&selfApp, 1, &kCFTypeArrayCallBacks);
	SecAccessRef access = NULL;
	SecAccessCreate(svc, trustedList, &access);
	CFRelease(trustedList);
	if (selfApp) CFRelease(selfApp);

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

	if (err == errSecDuplicateItem) {
		CFArrayRef searchList = CFArrayCreate(NULL, (const void **)&kref, 1, &kCFTypeArrayCallBacks);
		const void *qkeys[] = { kSecClass, kSecAttrService, kSecAttrAccount, kSecMatchSearchList };
		const void *qvals[] = { kSecClassGenericPassword, svc, acc, searchList };
		CFDictionaryRef query = CFDictionaryCreate(NULL, qkeys, qvals, 4,
		    &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
		const void *ukeys[] = { kSecValueData };
		const void *uvals[] = { dat };
		CFDictionaryRef upd = CFDictionaryCreate(NULL, ukeys, uvals, 1,
		    &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
		err = SecItemUpdate(query, upd);
		CFRelease(upd);
		CFRelease(query);
		CFRelease(searchList);
	}

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

func cgoGet(path, service, account string) (data []byte, found bool, err error) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	s := C.CString(service)
	defer C.free(unsafe.Pointer(s))
	a := C.CString(account)
	defer C.free(unsafe.Pointer(a))

	var outData unsafe.Pointer
	var outLen C.int
	code := C.kcGet(p, s, a, &outData, &outLen) //nolint:gocritic // false positive: out-param pattern
	switch code {
	case 0:
		b := C.GoBytes(outData, outLen)
		C.free(outData)
		return b, true, nil
	case 1:
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("SecItemCopyMatching: failed for %s", service)
	}
}

func cgoGetStatus(path string) (unlocked bool, err error) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	code := C.kcGetStatus(p)
	switch code {
	case 1:
		return true, nil
	case 0:
		return false, nil
	default:
		return false, fmt.Errorf("SecKeychainGetStatus: failed for %s", path)
	}
}

func cgoCountItems(path string) (int, error) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	code := C.kcCountItems(p)
	if code < 0 {
		return 0, fmt.Errorf("SecItemCopyMatching: failed for %s", path)
	}
	return int(code), nil
}

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

// keychainSettings holds the lock configuration of a keychain.
type keychainSettings struct {
	LockOnSleep  bool
	LockInterval uint32
}

func cgoGetSettings(path string) (keychainSettings, error) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	var lockOnSleep C.int
	var lockInterval C.uint
	if code := C.kcGetSettings(p, &lockOnSleep, &lockInterval); code != 0 {
		return keychainSettings{}, fmt.Errorf("SecKeychainCopySettings: failed for %s", path)
	}
	return keychainSettings{
		LockOnSleep:  lockOnSleep != 0,
		LockInterval: uint32(lockInterval),
	}, nil
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
