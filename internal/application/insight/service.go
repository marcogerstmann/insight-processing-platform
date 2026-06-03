package insight

import (
	"context"
	"errors"
	"strings"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

var errMissingID = errors.New("missing id")

type Result struct {
	Inserted bool
}

type InsightService interface {
	Process(ctx context.Context, ev domain.IngestEvent) (Result, error)
	ListByTenantID(ctx context.Context, tenantID string) ([]domain.Insight, error)
}

type Service struct {
	repo     ports.InsightRepository
	enricher ports.InsightEnricher
}

func NewService(repo ports.InsightRepository, enricher ports.InsightEnricher) *Service {
	return &Service{
		repo:     repo,
		enricher: enricher,
	}
}

var _ InsightService = (*Service)(nil)

func (s *Service) Process(ctx context.Context, ev domain.IngestEvent) (Result, error) {
	if strings.TrimSpace(ev.ID) == "" {
		return Result{}, errMissingID
	}

	i := s.buildInsight(ev)

	inserted, err := s.repo.PutIfAbsent(ctx, i)
	if err != nil {
		return Result{}, err
	}
	if !inserted {
		return Result{Inserted: false}, nil
	}

	if s.enricher == nil {
		return Result{Inserted: true}, nil
	}

	// TODO: enrichment is not yet implemented
	enriched, err := s.enricher.Enrich(ctx, i)
	if err != nil {
		return Result{}, err
	}

	if err := s.repo.Update(ctx, enriched); err != nil {
		return Result{}, err
	}

	return Result{Inserted: true}, nil
}

func (s *Service) ListByTenantID(ctx context.Context, tenantID string) ([]domain.Insight, error) {
	return s.repo.ListByTenantID(ctx, tenantID)
}

func (s *Service) buildInsight(ev domain.IngestEvent) domain.Insight {
	text := strings.TrimSpace(ev.Highlight.Text)

	return domain.Insight{
		ID:       ev.ID,
		TenantID: ev.TenantID,
		Source:   ev.Source,
		Text:     text,
	}
}
