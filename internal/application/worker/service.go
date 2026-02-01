package worker

import (
	"context"
	"strings"
	"time"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports/outbound/persistence"
)

type Service struct {
	repo persistence.InsightRepository
}

func NewService(repo persistence.InsightRepository) *Service {
	return &Service{
		repo: repo,
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
	text := strings.TrimSpace(ev.Highlight.Text)

	return domain.Insight{
		TenantID:       ev.TenantID,
		IdempotencyKey: ev.IdempotencyKey,
		Source:         ev.Source,
		Text:           text,
		CreatedAt:      time.Now(),
	}
}
