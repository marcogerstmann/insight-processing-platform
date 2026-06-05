package envutil

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

// ResolveSecret reads the env var named by key. If the value starts with "ssm:",
// the remainder is treated as an SSM parameter path and fetched via provider.
// Returns ("", nil) if the env var is unset or empty — callers decide whether
// absence is an error (required secrets) or acceptable (optional secrets).
func ResolveSecret(ctx context.Context, key string, provider ports.SecretProvider) (string, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return "", nil
	}
	if strings.HasPrefix(v, "ssm:") {
		if provider == nil {
			return "", fmt.Errorf("SSM provider required for %s but not configured", key)
		}
		return provider.Get(ctx, strings.TrimPrefix(v, "ssm:"))
	}
	return v, nil
}
