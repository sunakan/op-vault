//go:build darwin

package keychain

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/sunakan/op-keychain/internal/tracing"
)

// Remove removes a keychain at the given path.
func Remove(ctx context.Context, path string) error {
	_, span := tracing.Tracer().Start(ctx, "Remove")
	defer span.End()
	out, err := exec.CommandContext(ctx, "security", "delete-keychain", path).CombinedOutput() //nolint:gosec // path is a user-controlled input, not attacker-controlled
	if err != nil {
		tracing.SetSpanError(span, err)
		return fmt.Errorf("security delete-keychain: %w: %s", err, out)
	}
	return nil
}
