package tenant

import (
	"errors"
	"testing"

	"github.com/marcogerstmann/insight-processing-platform/internal/apperr"
)

func TestResolver_Resolve_Success(t *testing.T) {
	t.Setenv("DEFAULT_TENANT_ID", "tenant-foo")

	r := NewResolver()
	ctx, err := r.Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.TenantID != "tenant-foo" {
		t.Fatalf("tenant id mismatch: got %q want %q", ctx.TenantID, "tenant-foo")
	}
}

func TestResolver_Resolve_NoDefaultTenant(t *testing.T) {
	t.Setenv("DEFAULT_TENANT_ID", "")

	r := NewResolver()
	_, err := r.Resolve()
	if err == nil {
		t.Fatalf("expected server misconfigured error when DEFAULT_TENANT_ID is empty")
	}
	if !errors.Is(err, apperr.ErrServerMisconfigured) {
		t.Fatalf("unexpected error kind: got %v want apperr.ErrServerMisconfigured", err)
	}
}
