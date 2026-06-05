package readwise

import (
	"context"
	"errors"
	"testing"

	"github.com/marcogerstmann/insight-processing-platform/internal/apperr"
)

func TestWebhookAuthenticator_ValidSecret(t *testing.T) {
	t.Setenv("READWISE_WEBHOOK_SECRET", "s3cr3t")

	a := newWebhookAuthenticator(nil)
	if err := a.Authenticate("s3cr3t"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWebhookAuthenticator_InvalidSecret(t *testing.T) {
	t.Setenv("READWISE_WEBHOOK_SECRET", "expected")

	a := newWebhookAuthenticator(nil)
	err := a.Authenticate("wrong")
	if err == nil {
		t.Fatalf("expected unauthorized error for wrong secret")
	}
	if !errors.Is(err, apperr.ErrUnauthorized) {
		t.Fatalf("unexpected error kind: got %v want apperr.ErrUnauthorized", err)
	}
}

func TestWebhookAuthenticator_MissingSecret(t *testing.T) {
	t.Setenv("READWISE_WEBHOOK_SECRET", "")

	a := newWebhookAuthenticator(nil)
	err := a.Authenticate("irrelevant")
	if err == nil {
		t.Fatalf("expected server misconfigured error when webhook secret not configured")
	}
	if !errors.Is(err, apperr.ErrServerMisconfigured) {
		t.Fatalf("unexpected error kind: got %v want apperr.ErrServerMisconfigured", err)
	}
}

func TestWebhookAuthenticator_SSMPrefix(t *testing.T) {
	t.Setenv("READWISE_WEBHOOK_SECRET", "ssm:/test/path")

	a := newWebhookAuthenticator(&mockSecretProvider{value: "fetched-secret"})
	if err := a.Authenticate("fetched-secret"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

type mockSecretProvider struct {
	value string
	err   error
}

func (m *mockSecretProvider) Get(_ context.Context, _ string) (string, error) {
	return m.value, m.err
}
