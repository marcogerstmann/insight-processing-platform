package insight

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/marcogerstmann/insight-processing-platform/internal/apperr"
	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type Result struct {
	Inserted bool
}

type Service interface {
	Process(ctx context.Context, insight domain.Insight) (Result, error)
	ListByTenantID(ctx context.Context, tenantID string) ([]domain.Insight, error)
}

type service struct {
	repo     ports.InsightRepository
	enricher ports.InsightEnricher
}

func NewService(repo ports.InsightRepository, enricher ports.InsightEnricher) Service {
	return &service{
		repo:     repo,
		enricher: enricher,
	}
}

var _ Service = (*service)(nil)

func (s *service) Process(ctx context.Context, insight domain.Insight) (Result, error) {
	if strings.TrimSpace(insight.ID) == "" {
		return Result{}, apperr.PermanentError{Err: errors.New("missing id")}
	}

	inserted, err := s.repo.CreateIfAbsent(ctx, insight)
	if err != nil {
		return Result{}, err
	}
	if !inserted {
		return Result{Inserted: false}, nil
	}

	if s.enricher == nil {
		slog.WarnContext(ctx, "no enricher configured, skipping enrichment")
		return Result{Inserted: true}, nil
	}

	enriched, err := s.enricher.Enrich(ctx, insight)
	if err != nil {
		// Best-effort enrichment per ADR-006: LLM failure != system failure.
		slog.WarnContext(ctx, "enrichment failed, proceeding without summary", "err", err)
		return Result{Inserted: true}, nil
	}

	if err := s.repo.Update(ctx, enriched); err != nil {
		return Result{}, err
	}

	return Result{Inserted: true}, nil
}

func (s *service) ListByTenantID(ctx context.Context, tenantID string) ([]domain.Insight, error) {
	return s.repo.ListByTenantID(ctx, tenantID)
}
