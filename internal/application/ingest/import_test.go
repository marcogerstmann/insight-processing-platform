package ingest

import (
	"context"
	"encoding/json"
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

	result, err := im.Import(context.Background(), "tenant-1", "token", 0, false)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}
	if result.Fetched != 2 {
		t.Fatalf("expected Fetched=2 (blank text excluded before counting), got %d", result.Fetched)
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

	result, err := im.Import(context.Background(), "tenant-1", "token", 2, false)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}
	if result.Fetched != 2 || result.Enqueued != 2 {
		t.Fatalf("expected limit=2 to cap both Fetched and Enqueued, got %+v", result)
	}
}

func TestImport_OnlyFavoritesFiltersBeforeLimit(t *testing.T) {
	// Newest-first order, as FetchHighlights guarantees. Only "2" and "4" are
	// favorites — with onlyFavorites+limit=1, the result must be the single
	// most recent favorite ("2"), not a favorite among the top 1 overall
	// (which would find none, since "1" isn't a favorite).
	client := &fakeReadwiseClient{highlights: []ports.ReadwiseHighlight{
		{ID: "1", Text: "not favorite", IsFavorite: false},
		{ID: "2", Text: "favorite", IsFavorite: true},
		{ID: "3", Text: "not favorite", IsFavorite: false},
		{ID: "4", Text: "favorite", IsFavorite: true},
	}}
	mp := &mockPublisher{}
	svc := NewService(mp)
	im := NewImporter(client, svc)

	result, err := im.Import(context.Background(), "tenant-1", "token", 1, true)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}
	if result.Fetched != 1 || result.Enqueued != 1 {
		t.Fatalf("expected exactly 1 favorite enqueued, got %+v", result)
	}
	if mp.lastMsg.Attributes["idempotency_key"] != buildIdempotencyKey(domain.IngestEvent{
		TenantID:  "tenant-1",
		Source:    "readwise",
		EventType: "readwise.highlight.created",
		Highlight: domain.Highlight{ID: "2"},
	}) {
		t.Fatalf("expected the most recent favorite (id=2) to be the one enqueued")
	}
}

func TestImport_CarriesHighlightedAtFromReadwiseIntoEnqueuedEvent(t *testing.T) {
	highlightedAt := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)
	client := &fakeReadwiseClient{highlights: []ports.ReadwiseHighlight{
		{ID: "1", Text: "first", HighlightedAt: highlightedAt},
	}}
	mp := &mockPublisher{}
	svc := NewService(mp)
	im := NewImporter(client, svc)

	if _, err := im.Import(context.Background(), "tenant-1", "token", 0, false); err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	var ev domain.IngestEvent
	if err := json.Unmarshal(mp.lastMsg.Body, &ev); err != nil {
		t.Fatalf("unmarshal enqueued event: %v", err)
	}
	if !ev.Highlight.HighlightedAt.Equal(highlightedAt) {
		t.Fatalf("HighlightedAt = %v, want %v (Readwise's own highlight time)", ev.Highlight.HighlightedAt, highlightedAt)
	}
}

func TestImport_ClientErrorPropagated(t *testing.T) {
	client := &fakeReadwiseClient{err: errors.New("readwise down")}
	svc := NewService(&mockPublisher{})
	im := NewImporter(client, svc)

	_, err := im.Import(context.Background(), "tenant-1", "token", 0, false)
	if err == nil {
		t.Fatal("expected error to be propagated from client")
	}
}

func TestImport_SameHighlightSameEventTypeAsWebhook(t *testing.T) {
	client := &fakeReadwiseClient{highlights: []ports.ReadwiseHighlight{{ID: "42", Text: "text"}}}
	mp := &mockPublisher{}
	svc := NewService(mp)
	im := NewImporter(client, svc)

	if _, err := im.Import(context.Background(), "tenant-1", "token", 0, false); err != nil {
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
