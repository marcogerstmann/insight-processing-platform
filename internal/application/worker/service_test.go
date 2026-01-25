package worker

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mgerstmannsf/insight-processing-platform/internal/domain"
)

type mockRepo struct {
	called       bool
	gotInsight   domain.Insight
	wantInserted bool
	returnError  error
}

func (m *mockRepo) PutIfAbsent(_ context.Context, i domain.Insight) (bool, error) {
	m.called = true
	m.gotInsight = i
	return m.wantInserted, m.returnError
}

func TestService_Process_InsertAndFields(t *testing.T) {
	fixed := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	url := "https://example.com"
	ev := domain.IngestEvent{
		IdempotencyKey: "idem-key-1",
		TenantID:       "tenant-123",
		Source:         "source-a",
		EventType:      "ingest",
		ReceivedAt:     fixed.Add(-time.Minute),
		Highlight: domain.Highlight{
			ID:   456,
			Text: "  some highlighted text  ",
			Note: "\nnote content\n",
			URL:  &url,
			Tags: []string{"tag1", "tag2"},
		},
	}

	repo := &mockRepo{wantInserted: true}
	svc := &Service{
		repo: repo,
		now:  func() time.Time { return fixed },
	}

	res, err := svc.Process(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Inserted {
		t.Fatalf("expected Inserted true, got false")
	}
	if !repo.called {
		t.Fatalf("expected repo to be called")
	}

	got := repo.gotInsight

	if !got.CreatedAt.Equal(fixed) {
		t.Fatalf("CreatedAt: want %v, got %v", fixed, got.CreatedAt)
	}
	if !got.UpdatedAt.Equal(fixed) {
		t.Fatalf("UpdatedAt: want %v, got %v", fixed, got.UpdatedAt)
	}
	if got.IdempotencyKey != ev.IdempotencyKey {
		t.Fatalf("IdempotencyKey: want %q, got %q", ev.IdempotencyKey, got.IdempotencyKey)
	}
	if got.TenantID != ev.TenantID {
		t.Fatalf("TenantID: want %q, got %q", ev.TenantID, got.TenantID)
	}
	if got.HighlightID != ev.Highlight.ID {
		t.Fatalf("HighlightID: want %q, got %q", ev.Highlight.ID, got.HighlightID)
	}
	if got.Source != ev.Source {
		t.Fatalf("Source: want %q, got %q", ev.Source, got.Source)
	}
	if got.EventType != ev.EventType {
		t.Fatalf("EventType: want %q, got %q", ev.EventType, got.EventType)
	}
	if !got.ReceivedAt.Equal(ev.ReceivedAt) {
		t.Fatalf("ReceivedAt: want %v, got %v", ev.ReceivedAt, got.ReceivedAt)
	}
	if got.Text != "some highlighted text" {
		t.Fatalf("Text trimming: want %q, got %q", "some highlighted text", got.Text)
	}
	if got.Note != "note content" {
		t.Fatalf("Note trimming: want %q, got %q", "note content", got.Note)
	}
	if got.URL != *ev.Highlight.URL {
		t.Fatalf("URL: want %q, got %q", *ev.Highlight.URL, got.URL)
	}
	if !reflect.DeepEqual(got.Tags, ev.Highlight.Tags) {
		t.Fatalf("Tags: want %v, got %v", ev.Highlight.Tags, got.Tags)
	}
}

func TestService_Process_NotInserted(t *testing.T) {
	fixed := time.Now().UTC()
	url := "u"
	ev := domain.IngestEvent{
		IdempotencyKey: "k",
		TenantID:       "t",
		Source:         "s",
		EventType:      "e",
		ReceivedAt:     fixed,
		Highlight: domain.Highlight{
			ID:   123,
			Text: "x",
			URL:  &url,
			Tags: []string{},
		},
	}

	repo := &mockRepo{wantInserted: false}
	svc := &Service{repo: repo, now: func() time.Time { return fixed }}

	res, err := svc.Process(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Inserted {
		t.Fatalf("expected Inserted false, got true")
	}
	if !repo.called {
		t.Fatalf("expected repo to be called")
	}
}

func TestService_Process_RepoError(t *testing.T) {
	fixed := time.Now().UTC()
	url := "u"
	ev := domain.IngestEvent{
		IdempotencyKey: "k",
		TenantID:       "t",
		Highlight: domain.Highlight{
			ID:  123,
			URL: &url,
		},
	}

	repoErr := errors.New("db failure")
	repo := &mockRepo{returnError: repoErr}
	svc := &Service{repo: repo, now: func() time.Time { return fixed }}

	_, err := svc.Process(context.Background(), ev)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected error %v, got %v", repoErr, err)
	}
	if !repo.called {
		t.Fatalf("expected repo to be called")
	}
}
