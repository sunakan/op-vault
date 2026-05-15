package keychain

import (
	"crypto/sha256"
	"fmt"
)

func Service(ref string) string {
	hash := sha256.Sum256([]byte(ref))
	return fmt.Sprintf("op-keychain:%x", hash)
}
