#pragma clang diagnostic ignored "-Wdeprecated-declarations"

#include "keychain.h"
#include <string.h>

/* kcCreate creates a new keychain file at path, protected by password.
 * SecKeychainCreate is deprecated since macOS 12 — there is no non-deprecated
 * API to create a custom keychain file for a CLI tool. */
OSStatus
kcCreate(const char *path, const char *password, int pwLen)
{
	SecKeychainRef ref = NULL;
	OSStatus       err = SecKeychainCreate(path, (UInt32)pwLen, password,
	                                       (Boolean)0, NULL, &ref);
	if (ref)
		CFRelease(ref);
	return err;
}

/* kcDelete removes the keychain at path from the search list and deletes the file.
 * SecKeychainDelete does not free the ref — CFRelease must be called separately. */
OSStatus
kcDelete(const char *path)
{
	SecKeychainRef ref = NULL;
	OSStatus       err = SecKeychainOpen(path, &ref);
	if (err != noErr || ref == NULL)
		return err;
	err = SecKeychainDelete(ref);
	CFRelease(ref);
	return err;
}

/* kcOpen opens an existing keychain at path and returns its SecKeychainRef.
 * Caller must CFRelease the returned ref.
 * SecKeychainOpen is deprecated since macOS 12 — needed to target a specific
 * keychain file for SecItemAdd/SecItemCopyMatching via kSecUseKeychain /
 * kSecMatchSearchList. */
OSStatus
kcOpen(const char *path, SecKeychainRef *out)
{
	return SecKeychainOpen(path, out);
}

/* kcGetStatus returns 1 if unlocked, 0 if locked, -1 on error (sets *outErr). */
int
kcGetStatus(const char *path, OSStatus *outErr)
{
	*outErr = noErr;
	SecKeychainRef ref = NULL;
	if (SecKeychainOpen(path, &ref) != noErr || ref == NULL)
		return -1;
	SecKeychainStatus status = 0;
	OSStatus          err    = SecKeychainGetStatus(ref, &status);
	CFRelease(ref);
	if (err != noErr) {
		*outErr = err;
		return -1;
	}
	return (status & kSecUnlockStateStatus) ? 1 : 0;
}

/* kcCountItems returns the number of "1Password Cache" generic password items
 * in the keychain at path, or -1 on error (sets *outErr). */
int
kcCountItems(const char *path, OSStatus *outErr)
{
	*outErr = noErr;
	SecKeychainRef ref = NULL;
	if (SecKeychainOpen(path, &ref) != noErr || ref == NULL)
		return -1;
	CFStringRef desc       = CFStringCreateWithCString(NULL, "1Password Cache", kCFStringEncodingUTF8);
	CFArrayRef  searchList = CFArrayCreate(NULL, (const void **)&ref, 1, &kCFTypeArrayCallBacks);
	const void *keys[]     = { kSecClass, kSecAttrDescription, kSecMatchSearchList,
		                       kSecMatchLimit, kSecReturnAttributes };
	const void *vals[]     = { kSecClassGenericPassword, desc, searchList,
		                       kSecMatchLimitAll, kCFBooleanTrue };
	CFDictionaryRef query = CFDictionaryCreate(NULL, keys, vals, 5,
	                                           &kCFTypeDictionaryKeyCallBacks,
	                                           &kCFTypeDictionaryValueCallBacks);
	CFTypeRef result = NULL;
	OSStatus  err    = SecItemCopyMatching(query, &result);
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
		*outErr = err;
		count   = -1;
	}
	return count;
}

/* kcGet retrieves password data for service/account from the keychain at path.
 * Returns 0 (found, caller must free *outData), 1 (not found), or -1 (error, sets *outErr). */
int
kcGet(const char *path, const char *service, const char *account,
      void **outData, int *outLen, OSStatus *outErr)
{
	*outErr = noErr;
	SecKeychainRef ref = NULL;
	if (SecKeychainOpen(path, &ref) != noErr || ref == NULL)
		return -1;
	/* Try to unlock with an empty password before querying.
	 * Succeeds silently for no-password keychains — avoids the macOS dialog entirely.
	 * For password-protected keychains this fails; macOS then shows the dialog on
	 * the SecItemCopyMatching call below instead. */
	SecKeychainUnlock(ref, 0, (void *)"", TRUE);
	CFStringRef svc        = CFStringCreateWithCString(NULL, service, kCFStringEncodingUTF8);
	CFStringRef acc        = CFStringCreateWithCString(NULL, account, kCFStringEncodingUTF8);
	CFArrayRef  searchList = CFArrayCreate(NULL, (const void **)&ref, 1, &kCFTypeArrayCallBacks);
	const void *keys[]     = { kSecClass, kSecAttrService, kSecAttrAccount,
		                       kSecMatchSearchList, kSecMatchLimit, kSecReturnData };
	const void *vals[]     = { kSecClassGenericPassword, svc, acc,
		                       searchList, kSecMatchLimitOne, kCFBooleanTrue };
	CFDictionaryRef query  = CFDictionaryCreate(NULL, keys, vals, 6,
	                                            &kCFTypeDictionaryKeyCallBacks,
	                                            &kCFTypeDictionaryValueCallBacks);
	CFTypeRef result = NULL;
	OSStatus  err    = SecItemCopyMatching(query, &result);
	CFRelease(query);
	CFRelease(searchList);
	CFRelease(acc);
	CFRelease(svc);
	CFRelease(ref);
	if (err == errSecItemNotFound) {
		*outData = NULL;
		*outLen  = 0;
		return 1;
	}
	if (err != noErr || result == NULL) {
		*outErr = err;
		return -1;
	}
	CFDataRef dat = (CFDataRef)result;
	CFIndex   len = CFDataGetLength(dat);
	void     *buf = malloc((size_t)len);
	if (buf == NULL) {
		CFRelease(result);
		return -1;
	}
	memcpy(buf, CFDataGetBytePtr(dat), (size_t)len);
	CFRelease(result);
	*outData = buf;
	*outLen  = (int)len;
	return 0;
}

/* kcGetSettings retrieves lock settings from the keychain at path.
 * Returns 0 on success, -1 on error (sets *outErr).
 * SecKeychainCopySettings always sets useLockInterval=false on arm64 macOS 15+
 * even when a timeout is stored; callers should rely on lockInterval directly. */
int
kcGetSettings(const char *path, int *lockOnSleep,
              unsigned int *lockInterval, OSStatus *outErr)
{
	*outErr = noErr;
	SecKeychainRef ref = NULL;
	if (SecKeychainOpen(path, &ref) != noErr || ref == NULL)
		return -1;
	SecKeychainSettings settings = { SEC_KEYCHAIN_SETTINGS_VERS1, (Boolean)0, (Boolean)0, 0 };
	OSStatus            err      = SecKeychainCopySettings(ref, &settings);
	CFRelease(ref);
	if (err != noErr) {
		*outErr = err;
		return -1;
	}
	*lockOnSleep  = settings.lockOnSleep ? 1 : 0;
	*lockInterval = (unsigned int)settings.lockInterval;
	return 0;
}

/* kcClearItems deletes all "1Password Cache" items from the keychain at path.
 * Uses persistent refs to ensure only items from the target keychain are deleted.
 * Returns 0 on success (including when there are no items), -1 on error (sets *outErr). */
int
kcClearItems(const char *path, OSStatus *outErr)
{
	*outErr = noErr;
	SecKeychainRef ref = NULL;
	if (SecKeychainOpen(path, &ref) != noErr || ref == NULL) {
		*outErr = errSecNoSuchKeychain;
		return -1;
	}
	CFStringRef desc       = CFStringCreateWithCString(NULL, "1Password Cache", kCFStringEncodingUTF8);
	CFArrayRef  searchList = CFArrayCreate(NULL, (const void **)&ref, 1, &kCFTypeArrayCallBacks);
	const void *qkeys[]    = { kSecClass, kSecAttrDescription, kSecMatchSearchList,
		                       kSecMatchLimit, kSecReturnPersistentRef };
	const void *qvals[]    = { kSecClassGenericPassword, desc, searchList,
		                       kSecMatchLimitAll, kCFBooleanTrue };
	CFDictionaryRef query  = CFDictionaryCreate(NULL, qkeys, qvals, 5,
	                                            &kCFTypeDictionaryKeyCallBacks,
	                                            &kCFTypeDictionaryValueCallBacks);
	CFTypeRef result = NULL;
	OSStatus  err    = SecItemCopyMatching(query, &result);
	CFRelease(query);
	CFRelease(searchList);
	CFRelease(desc);
	CFRelease(ref);
	if (err == errSecItemNotFound)
		return 0;
	if (err != noErr || result == NULL) {
		*outErr = err;
		return -1;
	}
	CFArrayRef items     = (CFArrayRef)result;
	CFIndex    n         = CFArrayGetCount(items);
	OSStatus   deleteErr = noErr;
	for (CFIndex i = 0; i < n; i++) {
		CFDataRef   persistRef = (CFDataRef)CFArrayGetValueAtIndex(items, i);
		const void *dkeys[]    = { kSecValuePersistentRef };
		const void *dvals[]    = { persistRef };
		CFDictionaryRef dq = CFDictionaryCreate(NULL, dkeys, dvals, 1,
		                                        &kCFTypeDictionaryKeyCallBacks,
		                                        &kCFTypeDictionaryValueCallBacks);
		OSStatus e = SecItemDelete(dq);
		CFRelease(dq);
		if (e != noErr && e != errSecItemNotFound)
			deleteErr = e;
	}
	CFRelease(result);
	if (deleteErr != noErr) {
		*outErr = deleteErr;
		return -1;
	}
	return 0;
}

/* kcAdd adds or updates a generic password in the keychain referenced by kref.
 * On errSecDuplicateItem, falls back to SecItemUpdate to overwrite the value.
 * kSecAttrAccess restricts reads to the current binary only — prevents other processes
 * from silently reading cached 1Password secrets.
 * NULL trusted list (any app can read) was the previous approach; replaced to limit
 * silent access to cached secrets. */
OSStatus
kcAdd(SecKeychainRef kref, const char *service, const char *account,
      const char *description, const char *label,
      const void *data, int dataLen)
{
	CFStringRef svc  = CFStringCreateWithCString(NULL, service,     kCFStringEncodingUTF8);
	CFStringRef acc  = CFStringCreateWithCString(NULL, account,     kCFStringEncodingUTF8);
	CFStringRef desc = CFStringCreateWithCString(NULL, description, kCFStringEncodingUTF8);
	CFStringRef lbl  = CFStringCreateWithCString(NULL, label,       kCFStringEncodingUTF8);
	CFDataRef   dat  = CFDataCreate(NULL, (const UInt8 *)data, (CFIndex)dataLen);

	SecTrustedApplicationRef selfApp = NULL;
	SecTrustedApplicationCreateFromPath(NULL, &selfApp);
	CFArrayRef   trustedList = CFArrayCreate(NULL, (const void **)&selfApp, 1, &kCFTypeArrayCallBacks);
	SecAccessRef access      = NULL;
	SecAccessCreate(svc, trustedList, &access);
	CFRelease(trustedList);
	if (selfApp)
		CFRelease(selfApp);

	const void *keys[] = { kSecClass, kSecAttrService, kSecAttrAccount,
		                   kSecAttrDescription, kSecAttrLabel, kSecValueData,
		                   kSecUseKeychain, kSecAttrAccess };
	const void *vals[] = { kSecClassGenericPassword, svc, acc, desc, lbl, dat, kref, access };
	CFDictionaryRef attrs = CFDictionaryCreate(NULL, keys, vals, 8,
	                                           &kCFTypeDictionaryKeyCallBacks,
	                                           &kCFTypeDictionaryValueCallBacks);

	OSStatus err = SecItemAdd(attrs, NULL);

	if (err == errSecDuplicateItem) {
		CFArrayRef  searchList = CFArrayCreate(NULL, (const void **)&kref, 1, &kCFTypeArrayCallBacks);
		const void *qkeys[]   = { kSecClass, kSecAttrService, kSecAttrAccount, kSecMatchSearchList };
		const void *qvals[]   = { kSecClassGenericPassword, svc, acc, searchList };
		CFDictionaryRef query  = CFDictionaryCreate(NULL, qkeys, qvals, 4,
		                                            &kCFTypeDictionaryKeyCallBacks,
		                                            &kCFTypeDictionaryValueCallBacks);
		/* kSecAttrAccess is intentionally omitted from the update dict — including it
		 * overwrites the existing access control and triggers multiple confirmation dialogs. */
		const void *ukeys[]   = { kSecValueData };
		const void *uvals[]   = { dat };
		CFDictionaryRef upd    = CFDictionaryCreate(NULL, ukeys, uvals, 1,
		                                            &kCFTypeDictionaryKeyCallBacks,
		                                            &kCFTypeDictionaryValueCallBacks);
		err = SecItemUpdate(query, upd);
		CFRelease(upd);
		CFRelease(query);
		CFRelease(searchList);
	}

	CFRelease(attrs);
	if (access)
		CFRelease(access);
	CFRelease(dat);
	CFRelease(lbl);
	CFRelease(desc);
	CFRelease(acc);
	CFRelease(svc);
	return err;
}

/* kcFreeList frees parallel ref/date arrays returned by kcList. */
void
kcFreeList(char **refs, char **dates, int count)
{
	for (int i = 0; i < count; i++) {
		free(refs[i]);
		free(dates[i]);
	}
	free(refs);
	free(dates);
}

/* kcList returns two parallel malloc'd char** arrays (refs and dates) of length *outCount,
 * or NULL on empty/error. dates[i] is "YYYY-MM-DD HH:MM:SS" (local time) from
 * kSecAttrModificationDate, or "" if absent. Caller must call kcFreeList. */
char **
kcList(const char *path, char ***outDates, int *outCount, OSStatus *outErr)
{
	*outErr   = noErr;
	*outCount = 0;
	*outDates = NULL;
	SecKeychainRef ref = NULL;
	if (SecKeychainOpen(path, &ref) != noErr || ref == NULL)
		return NULL;
	CFStringRef desc       = CFStringCreateWithCString(NULL, "1Password Cache", kCFStringEncodingUTF8);
	CFArrayRef  searchList = CFArrayCreate(NULL, (const void **)&ref, 1, &kCFTypeArrayCallBacks);
	const void *keys[]     = { kSecClass, kSecAttrDescription, kSecMatchSearchList,
		                       kSecMatchLimit, kSecReturnAttributes };
	const void *vals[]     = { kSecClassGenericPassword, desc, searchList,
		                       kSecMatchLimitAll, kCFBooleanTrue };
	CFDictionaryRef query  = CFDictionaryCreate(NULL, keys, vals, 5,
	                                            &kCFTypeDictionaryKeyCallBacks,
	                                            &kCFTypeDictionaryValueCallBacks);
	CFTypeRef result = NULL;
	OSStatus  err    = SecItemCopyMatching(query, &result);
	CFRelease(query);
	CFRelease(searchList);
	CFRelease(desc);
	CFRelease(ref);
	if (err == errSecItemNotFound || result == NULL)
		return NULL;
	if (err != noErr) {
		*outErr = err;
		return NULL;
	}
	CFArrayRef arr   = (CFArrayRef)result;
	int        count = (int)CFArrayGetCount(arr);
	char     **refs  = (char **)malloc((size_t)count * sizeof(char *));
	char     **dates = (char **)malloc((size_t)count * sizeof(char *));
	if (refs == NULL || dates == NULL) {
		free(refs);
		free(dates);
		CFRelease(result);
		return NULL;
	}
	for (int i = 0; i < count; i++) {
		refs[i]  = NULL;
		dates[i] = NULL;
	}
	for (int i = 0; i < count; i++) {
		CFDictionaryRef item = (CFDictionaryRef)CFArrayGetValueAtIndex(arr, i);
		CFStringRef     svc  = (CFStringRef)CFDictionaryGetValue(item, kSecAttrService);
		if (svc == NULL) {
			refs[i] = strdup("");
		} else {
			CFIndex len = CFStringGetMaximumSizeForEncoding(
			                  CFStringGetLength(svc), kCFStringEncodingUTF8) + 1;
			char *buf = (char *)malloc((size_t)len);
			if (buf != NULL)
				CFStringGetCString(svc, buf, len, kCFStringEncodingUTF8);
			refs[i] = (buf != NULL) ? buf : strdup("");
		}
		if (refs[i] == NULL) {
			kcFreeList(refs, dates, i);
			CFRelease(result);
			return NULL;
		}
		CFDateRef modDate = (CFDateRef)CFDictionaryGetValue(item, kSecAttrModificationDate);
		if (modDate == NULL) {
			dates[i] = strdup("");
		} else {
			/* CFAbsoluteTime: seconds since 2001-01-01 UTC; Unix epoch offset = 978307200 */
			time_t     t      = (time_t)(CFDateGetAbsoluteTime(modDate) + 978307200.0);
			struct tm  tm_buf;
			localtime_r(&t, &tm_buf);
			char *buf = (char *)malloc(20);
			if (buf != NULL)
				strftime(buf, 20, "%Y-%m-%d %H:%M:%S", &tm_buf);
			dates[i] = (buf != NULL) ? buf : strdup("");
		}
		if (dates[i] == NULL) {
			kcFreeList(refs, dates, i + 1);
			CFRelease(result);
			return NULL;
		}
	}
	CFRelease(result);
	*outCount = count;
	*outDates = dates;
	return refs;
}
