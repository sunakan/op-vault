//go:build darwin

package keychain

// NotFoundError is returned when the keychain file does not exist.
type NotFoundError struct {
	Path string
}

func (e *NotFoundError) Error() string {
	return "keychain not found: " + e.Path
}

// CacheMissError is returned when no cache entry exists for the given ref.
type CacheMissError struct {
	Ref string
}

func (e *CacheMissError) Error() string {
	return "cache miss: " + e.Ref
}
