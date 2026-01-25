package tenant

import (
	"errors"
	"testing"

	"github.com/mgerstmannsf/insight-processing-platform/internal/application/apperr"
)

func TestResolver_Resolve_Success(t *testing.T) {
	t.Setenv("DEFAULT_TENANT_ID", "tenant-foo")
	t.Setenv("READWISE_WEBHOOK_SECRET", "s3cr3t")

	r := NewResolver()
	ctx, err := r.Resolve(ResolveInput{Source: "readwise", Secret: "s3cr3t"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.TenantID != "tenant-foo" {
		t.Fatalf("tenant id mismatch: got %q want %q", ctx.TenantID, "tenant-foo")
	}
}

func TestResolver_Resolve_NoWebhookSecret(t *testing.T) {
	t.Setenv("DEFAULT_TENANT_ID", "tenant-foo")
	t.Setenv("READWISE_WEBHOOK_SECRET", "")

	r := NewResolver()
	_, err := r.Resolve(ResolveInput{Source: "readwise", Secret: "irrelevant"})
	if err == nil {
		t.Fatalf("expected unauthorized error when webhook secret not configured")
	}
	if !errors.Is(err, apperr.ErrUnauthorized) {
		t.Fatalf("unexpected error kind: got %v want apperr.ErrUnauthorized", err)
	}
}

func TestResolver_Resolve_BadSecret(t *testing.T) {
	t.Setenv("DEFAULT_TENANT_ID", "tenant-foo")
	t.Setenv("READWISE_WEBHOOK_SECRET", "expected")

	r := NewResolver()
	_, err := r.Resolve(ResolveInput{Source: "readwise", Secret: "wrong"})
	if err == nil {
		t.Fatalf("expected unauthorized error for wrong secret")
	}
	if !errors.Is(err, apperr.ErrUnauthorized) {
		t.Fatalf("unexpected error kind: got %v want apperr.ErrUnauthorized", err)
	}
}

func TestResolver_Resolve_NoDefaultTenant(t *testing.T) {
	t.Setenv("DEFAULT_TENANT_ID", "")
	t.Setenv("READWISE_WEBHOOK_SECRET", "s3cr3t")

	r := NewResolver()
	_, err := r.Resolve(ResolveInput{Source: "readwise", Secret: "s3cr3t"})
	if err == nil {
		t.Fatalf("expected server misconfigured error when DEFAULT_TENANT_ID is empty")
	}
	if !errors.Is(err, apperr.ErrServerMisconfigured) {
		t.Fatalf("unexpected error kind: got %v want apperr.ErrServerMisconfigured", err)
	}
}

func TestResolver_Resolve_UnknownSource(t *testing.T) {
	t.Setenv("DEFAULT_TENANT_ID", "tenant-foo")
	t.Setenv("READWISE_WEBHOOK_SECRET", "s3cr3t")

	r := NewResolver()
	_, err := r.Resolve(ResolveInput{Source: "unknown", Secret: "does-not-matter"})
	if err == nil {
		t.Fatalf("expected unauthorized error for unknown source")
	}
	if !errors.Is(err, apperr.ErrUnauthorized) {
		t.Fatalf("unexpected error kind: got %v want apperr.ErrUnauthorized", err)
	}
}
