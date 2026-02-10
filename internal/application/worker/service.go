package worker

import (
	"context"
	"errors"
	"strings"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports/outbound"
)

var errMissingIdempotencyKey = errors.New("missing idempotency key")

type Service struct {
	repo     outbound.InsightRepository
	enricher outbound.InsightEnricher
}

func NewService(repo outbound.InsightRepository, enricher outbound.InsightEnricher) *Service {
	return &Service{
		repo:     repo,
		enricher: enricher,
	}
}

type Result struct {
	Inserted bool
}

func (s *Service) Process(ctx context.Context, ev domain.IngestEvent) (Result, error) {
	if strings.TrimSpace(ev.IdempotencyKey) == "" {
		return Result{}, errMissingIdempotencyKey
	}

	insight := s.buildInsight(ev)

	inserted, err := s.repo.PutIfAbsent(ctx, insight)
	if err != nil {
		return Result{}, err
	}
	if !inserted {
		return Result{Inserted: false}, nil
	}

	if s.enricher == nil {
		return Result{Inserted: true}, nil
	}

	enriched, err := s.enricher.Enrich(ctx, insight)
	if err != nil {
		return Result{}, err
	}

	if err := s.repo.Update(ctx, enriched); err != nil {
		return Result{}, err
	}

	return Result{Inserted: true}, nil
}

func (s *Service) buildInsight(ev domain.IngestEvent) domain.Insight {
	text := strings.TrimSpace(ev.Highlight.Text)

	return domain.Insight{
		TenantID:       ev.TenantID,
		IdempotencyKey: ev.IdempotencyKey,
		Source:         ev.Source,
		Text:           text,
	}
}
