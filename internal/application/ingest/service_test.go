package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports/outbound"
)

type mockPublisher struct {
	lastMsg     outbound.PublishMessage
	called      bool
	errToReturn error
}

func (f *mockPublisher) Publish(_ context.Context, m outbound.PublishMessage) error {
	f.called = true
	f.lastMsg = m
	return f.errToReturn
}

func TestEnqueueReadwise_PublishesMessage(t *testing.T) {
	ctx := context.Background()
	mp := &mockPublisher{}
	s := NewService(mp)

	updatedAt := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)

	ingestEvent := domain.IngestEvent{
		Source:    "readwise",
		EventType: "create",
		Highlight: domain.Highlight{
			ID:        42,
			UpdatedAt: updatedAt,
		},
	}

	tenantID := "tenant-123"

	err := s.EnqueueReadwise(ctx, ingestEvent, tenantID)
	if err != nil {
		t.Fatalf("EnqueueReadwise returned unexpected error: %v", err)
	}

	if !mp.called {
		t.Fatalf("expected publisher to be called")
	}

	msg := mp.lastMsg

	evForKey := ingestEvent
	evForKey.TenantID = tenantID
	evForKey.IdempotencyKey = buildIdempotencyKey(evForKey)
	expectedBody, err := json.Marshal(evForKey)
	if err != nil {
		t.Fatalf("failed to marshal expected body: %v", err)
	}
	if string(msg.Body) != string(expectedBody) {
		t.Fatalf("body mismatch: got %s want %s", string(msg.Body), string(expectedBody))
	}

	attrs := msg.Attributes
	if attrs["tenantId"] != tenantID {
		t.Fatalf("tenantId attr mismatch: got %q want %q", attrs["tenantId"], tenantID)
	}
	expectedIdem := buildIdempotencyKey(evForKey)
	if attrs["idempotencyKey"] != expectedIdem {
		t.Fatalf("idempotencyKey attr mismatch: got %q want %q", attrs["idempotencyKey"], expectedIdem)
	}
}

func TestEnqueueReadwise_PublisherErrorPropagated(t *testing.T) {
	ctx := context.Background()
	mp := &mockPublisher{errToReturn: errors.New("publish failed")}
	s := NewService(mp)

	ev := domain.IngestEvent{
		Source:    "readwise",
		EventType: "create",
		Highlight: domain.Highlight{
			ID:        1,
			UpdatedAt: time.Now(),
		},
	}

	err := s.EnqueueReadwise(ctx, ev, "tenant-x")
	if err == nil {
		t.Fatalf("expected error to be propagated from publisher")
	}
	if err.Error() != "publish failed" {
		t.Fatalf("unexpected error: got %v", err)
	}
	if !mp.called {
		t.Fatalf("expected publisher to be called even when it returns an error")
	}
}
