package ingest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type fakeReadwiseClient struct {
	highlights []ports.ReadwiseHighlight
	err        error
}

func (f *fakeReadwiseClient) FetchHighlights(_ context.Context, _ string) ([]ports.ReadwiseHighlight, error) {
	return f.highlights, f.err
}

func TestImport_EnqueuesAllByDefault(t *testing.T) {
	client := &fakeReadwiseClient{highlights: []ports.ReadwiseHighlight{
		{ID: "1", Text: "first", HighlightedAt: time.Now()},
		{ID: "2", Text: "second", HighlightedAt: time.Now()},
		{ID: "3", Text: "  ", HighlightedAt: time.Now()}, // blank text, skipped
	}}
	mp := &mockPublisher{}
	svc := NewService(mp)
	im := NewImporter(client, svc)

	result, err := im.Import(context.Background(), "tenant-1", "token", 0)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}
	if result.Fetched != 3 {
		t.Fatalf("expected Fetched=3, got %d", result.Fetched)
	}
	if result.Enqueued != 2 {
		t.Fatalf("expected Enqueued=2 (blank text skipped), got %d", result.Enqueued)
	}
}

func TestImport_RespectsLimit(t *testing.T) {
	client := &fakeReadwiseClient{highlights: []ports.ReadwiseHighlight{
		{ID: "1", Text: "first"},
		{ID: "2", Text: "second"},
		{ID: "3", Text: "third"},
	}}
	mp := &mockPublisher{}
	svc := NewService(mp)
	im := NewImporter(client, svc)

	result, err := im.Import(context.Background(), "tenant-1", "token", 2)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}
	if result.Fetched != 2 || result.Enqueued != 2 {
		t.Fatalf("expected limit=2 to cap both Fetched and Enqueued, got %+v", result)
	}
}

func TestImport_ClientErrorPropagated(t *testing.T) {
	client := &fakeReadwiseClient{err: errors.New("readwise down")}
	svc := NewService(&mockPublisher{})
	im := NewImporter(client, svc)

	_, err := im.Import(context.Background(), "tenant-1", "token", 0)
	if err == nil {
		t.Fatal("expected error to be propagated from client")
	}
}

func TestImport_SameHighlightSameEventTypeAsWebhook(t *testing.T) {
	client := &fakeReadwiseClient{highlights: []ports.ReadwiseHighlight{{ID: "42", Text: "text"}}}
	mp := &mockPublisher{}
	svc := NewService(mp)
	im := NewImporter(client, svc)

	if _, err := im.Import(context.Background(), "tenant-1", "token", 0); err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if mp.lastMsg.Attributes["idempotency_key"] == "" {
		t.Fatal("expected idempotency_key attribute to be set")
	}

	// Must match what the webhook path (apigw/readwise) would produce for the
	// same highlight, so the two ingestion routes dedupe against each other.
	want := buildIdempotencyKey(domain.IngestEvent{
		TenantID:  "tenant-1",
		Source:    "readwise",
		EventType: "readwise.highlight.created",
		Highlight: domain.Highlight{ID: "42"},
	})
	if got := mp.lastMsg.Attributes["idempotency_key"]; got != want {
		t.Fatalf("idempotency_key mismatch: got %q want %q (would not dedupe with webhook-delivered highlights)", got, want)
	}
}
