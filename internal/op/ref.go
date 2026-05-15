package op

import (
	"fmt"
	"strings"
)

type Ref struct {
	Vault string
	Item  string
	Field string
}

// ParseRef parses and validates an op:// reference.
// Returns error with exit code 2 semantics — caller should treat it as invalid input.
func ParseRef(ref string) (Ref, error) {
	if !strings.HasPrefix(ref, "op://") {
		return Ref{}, fmt.Errorf("invalid ref format: %s", ref)
	}
	parts := strings.SplitN(strings.TrimPrefix(ref, "op://"), "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return Ref{}, fmt.Errorf("invalid ref format: %s", ref)
	}
	r := Ref{Vault: parts[0], Item: parts[1]}
	if len(parts) == 3 {
		r.Field = parts[2]
	}
	return r, nil
}
