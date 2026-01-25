package worker

import (
	"context"
	"strings"
	"time"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports/outbound"
)

type Service struct {
	repo outbound.InsightRepository
	now  func() time.Time
}

func NewService(repo outbound.InsightRepository) *Service {
	return &Service{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

type Result struct {
	Inserted bool
}

func (s *Service) Process(ctx context.Context, ev domain.IngestEvent) (Result, error) {
	insight := s.buildInsight(ev)

	inserted, err := s.repo.PutIfAbsent(ctx, insight)
	if err != nil {
		return Result{}, err
	}

	return Result{Inserted: inserted}, nil
}

func (s *Service) buildInsight(ev domain.IngestEvent) domain.Insight {
	now := s.now()

	text := strings.TrimSpace(ev.Highlight.Text)
	note := strings.TrimSpace(ev.Highlight.Note)

	return domain.Insight{
		CreatedAt:      now,
		UpdatedAt:      now,
		IdempotencyKey: ev.IdempotencyKey,
		TenantID:       ev.TenantID,
		HighlightID:    ev.Highlight.ID,
		Source:         ev.Source,
		EventType:      ev.EventType,
		ReceivedAt:     ev.ReceivedAt,
		Text:           text,
		Note:           note,
		URL:            *ev.Highlight.URL,
		Tags:           ev.Highlight.Tags,
	}
}
