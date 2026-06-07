#pragma once

#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>
#include <stdlib.h>
#include <time.h>

/* SecKeychainCopySettings: present in Security.framework binary (verified via
 * .tbd) but absent from public SDK headers — forward-declare to call it
 * directly. */
extern OSStatus SecKeychainCopySettings(SecKeychainRef keychain,
                                        SecKeychainSettings *outSettings);

OSStatus kcCreate(const char *path, const char *password, int pwLen);
OSStatus kcDelete(const char *path);
OSStatus kcOpen(const char *path, SecKeychainRef *out);
int kcGetStatus(const char *path, OSStatus *outErr);
int kcCountItems(const char *path, OSStatus *outErr);
int kcGet(const char *path, const char *service, const char *account,
          void **outData, int *outLen, OSStatus *outErr);
int kcGetSettings(const char *path, int *lockOnSleep,
                  unsigned int *lockInterval, OSStatus *outErr);
int kcClearItems(const char *path, OSStatus *outErr);
OSStatus kcAdd(SecKeychainRef kref, const char *service, const char *account,
               const char *description, const char *label, const void *data,
               int dataLen);
void kcFreeList(char **refs, char **dates, int count);
char **kcList(const char *path, char ***outDates, int *outCount,
              OSStatus *outErr);
