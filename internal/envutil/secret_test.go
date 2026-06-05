package envutil

import (
	"context"
	"errors"
	"testing"
)

func TestResolveSecret_LiteralValue(t *testing.T) {
	t.Setenv("MY_SECRET", "literal-value")

	got, err := ResolveSecret(context.Background(), "MY_SECRET", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "literal-value" {
		t.Fatalf("got %q want %q", got, "literal-value")
	}
}

func TestResolveSecret_Empty(t *testing.T) {
	t.Setenv("MY_SECRET", "")

	got, err := ResolveSecret(context.Background(), "MY_SECRET", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty string for unset env var")
	}
}

func TestResolveSecret_SSMPrefix(t *testing.T) {
	t.Setenv("MY_SECRET", "ssm:/test/path")

	provider := &stubProvider{value: "fetched-from-ssm"}
	got, err := ResolveSecret(context.Background(), "MY_SECRET", provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "fetched-from-ssm" {
		t.Fatalf("got %q want %q", got, "fetched-from-ssm")
	}
	if provider.calledWith != "/test/path" {
		t.Fatalf("SSM called with %q want %q", provider.calledWith, "/test/path")
	}
}

func TestResolveSecret_SSMPrefix_NoProvider(t *testing.T) {
	t.Setenv("MY_SECRET", "ssm:/test/path")

	_, err := ResolveSecret(context.Background(), "MY_SECRET", nil)
	if err == nil {
		t.Fatalf("expected error when SSM prefix used without provider")
	}
}

func TestResolveSecret_SSMPrefix_FetchError(t *testing.T) {
	t.Setenv("MY_SECRET", "ssm:/test/path")

	provider := &stubProvider{err: errors.New("ssm fetch failed")}
	_, err := ResolveSecret(context.Background(), "MY_SECRET", provider)
	if err == nil {
		t.Fatalf("expected error when SSM fetch fails")
	}
}

type stubProvider struct {
	value      string
	err        error
	calledWith string
}

func (s *stubProvider) Get(_ context.Context, name string) (string, error) {
	s.calledWith = name
	return s.value, s.err
}
