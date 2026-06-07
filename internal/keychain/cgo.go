//go:build darwin

package keychain

/*
#cgo LDFLAGS: -framework CoreFoundation -framework Security
#pragma clang diagnostic ignored "-Wdeprecated-declarations"

#include "keychain.h"
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
	var outErr C.OSStatus
	code := C.kcGet(p, s, a, &outData, &outLen, &outErr) //nolint:gocritic // false positive: out-param pattern
	switch code {
	case 0:
		b := C.GoBytes(outData, outLen)
		C.free(outData)
		return b, true, nil
	case 1:
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("SecItemCopyMatching: %s (service=%s)", osStatusString(int(outErr)), service)
	}
}

func cgoGetStatus(path string) (unlocked bool, err error) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	var outErr C.OSStatus
	code := C.kcGetStatus(p, &outErr)
	switch code {
	case 1:
		return true, nil
	case 0:
		return false, nil
	default:
		return false, fmt.Errorf("SecKeychainGetStatus: %s", osStatusString(int(outErr)))
	}
}

func cgoCountItems(path string) (int, error) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	var outErr C.OSStatus
	code := C.kcCountItems(p, &outErr)
	if code < 0 {
		return 0, fmt.Errorf("SecItemCopyMatching: %s", osStatusString(int(outErr)))
	}
	return int(code), nil
}

func cgoList(path string) ([]ListEntry, error) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	var outErr C.OSStatus
	var outCount C.int
	var outAccounts **C.char
	var outDates **C.char
	refs := C.kcList(p, &outAccounts, &outDates, &outCount, &outErr) //nolint:gocritic // false positive: CGo out-param pattern
	if outErr != 0 {
		return nil, fmt.Errorf("kcList: %s", osStatusString(int(outErr)))
	}
	if refs == nil || outCount == 0 {
		return nil, nil
	}
	defer C.kcFreeList(refs, outAccounts, outDates, outCount)
	count := int(outCount)
	refSlice := (*[1 << 20]*C.char)(unsafe.Pointer(refs))[:count:count]
	accountSlice := (*[1 << 20]*C.char)(unsafe.Pointer(outAccounts))[:count:count]
	dateSlice := (*[1 << 20]*C.char)(unsafe.Pointer(outDates))[:count:count]
	entries := make([]ListEntry, count)
	for i := range entries {
		entries[i] = ListEntry{
			Ref:       C.GoString(refSlice[i]),
			Account:   C.GoString(accountSlice[i]),
			UpdatedAt: C.GoString(dateSlice[i]),
		}
	}
	return entries, nil
}

func cgoCreate(path, password string) error {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	pw := C.CString(password)
	defer C.free(unsafe.Pointer(pw))
	if code := C.kcCreate(p, pw, C.int(len(password))); code != 0 {
		return fmt.Errorf("SecKeychainCreate: %s", osStatusString(int(code)))
	}
	return nil
}

func cgoDelete(path string) error {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	if code := C.kcDelete(p); code != 0 {
		return fmt.Errorf("SecKeychainDelete: %s", osStatusString(int(code)))
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
	var outErr C.OSStatus
	if code := C.kcGetSettings(p, &lockOnSleep, &lockInterval, &outErr); code != 0 {
		return keychainSettings{}, fmt.Errorf("SecKeychainCopySettings: %s", osStatusString(int(outErr)))
	}
	return keychainSettings{
		LockOnSleep:  lockOnSleep != 0,
		LockInterval: uint32(lockInterval),
	}, nil
}

func cgoClearItems(path string) error {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	var outErr C.OSStatus
	if code := C.kcClearItems(p, &outErr); code != 0 {
		return fmt.Errorf("SecItemDelete: %s", osStatusString(int(outErr)))
	}
	return nil
}

func cgoAdd(path, service, account, description, label string, data []byte) error {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))

	var kref C.SecKeychainRef
	if code := C.kcOpen(p, &kref); code != 0 { //nolint:gocritic // false positive: gocritic misidentifies CGO out-param pattern as dupSubExpr
		return fmt.Errorf("SecKeychainOpen: %s", osStatusString(int(code)))
	}
	defer C.CFRelease(C.CFTypeRef(kref))

	svc := C.CString(service)
	defer C.free(unsafe.Pointer(svc))
	acc := C.CString(account)
	defer C.free(unsafe.Pointer(acc))
	desc := C.CString(description)
	defer C.free(unsafe.Pointer(desc))
	lbl := C.CString(label)
	defer C.free(unsafe.Pointer(lbl))

	var dataPtr unsafe.Pointer
	if len(data) > 0 {
		dataPtr = unsafe.Pointer(&data[0])
	}
	if code := C.kcAdd(kref, svc, acc, desc, lbl, dataPtr, C.int(len(data))); code != 0 {
		return fmt.Errorf("SecItemAdd: %s", osStatusString(int(code)))
	}
	return nil
}
