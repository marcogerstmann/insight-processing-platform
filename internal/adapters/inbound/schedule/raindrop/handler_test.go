package raindrop

import (
	"context"
	"errors"
	"testing"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type fakeHighlightSource struct {
	highlights []ports.SourceHighlight
	err        error
}

func (f *fakeHighlightSource) FetchHighlights(_ context.Context) ([]ports.SourceHighlight, error) {
	return f.highlights, f.err
}

type fakeService struct{ enqueued int }

func (f *fakeService) Enqueue(_ context.Context, _ domain.IngestEvent) error {
	f.enqueued++
	return nil
}

func TestPoll_Success(t *testing.T) {
	source := &fakeHighlightSource{highlights: []ports.SourceHighlight{
		{ID: "1", Text: "first"},
		{ID: "2", Text: "second"},
	}}
	svc := &fakeService{}
	h := NewHandler(source, svc, "tenant-1", 50)

	if err := h.Poll(context.Background()); err != nil {
		t.Fatalf("Poll returned error: %v", err)
	}
	if svc.enqueued != 2 {
		t.Fatalf("expected 2 enqueued, got %d", svc.enqueued)
	}
}

func TestPoll_RespectsLimit(t *testing.T) {
	source := &fakeHighlightSource{highlights: []ports.SourceHighlight{
		{ID: "1", Text: "first"},
		{ID: "2", Text: "second"},
		{ID: "3", Text: "third"},
	}}
	svc := &fakeService{}
	h := NewHandler(source, svc, "tenant-1", 1)

	if err := h.Poll(context.Background()); err != nil {
		t.Fatalf("Poll returned error: %v", err)
	}
	if svc.enqueued != 1 {
		t.Fatalf("expected limit=1 to cap enqueued, got %d", svc.enqueued)
	}
}

func TestPoll_UpstreamErrorPropagated(t *testing.T) {
	source := &fakeHighlightSource{err: errors.New("raindrop: invalid API token")}
	svc := &fakeService{}
	h := NewHandler(source, svc, "tenant-1", 50)

	err := h.Poll(context.Background())
	if err == nil {
		t.Fatal("expected error to be returned so the Lambda invocation is recorded as failed")
	}
	if svc.enqueued != 0 {
		t.Fatalf("expected nothing enqueued on upstream failure, got %d", svc.enqueued)
	}
}
