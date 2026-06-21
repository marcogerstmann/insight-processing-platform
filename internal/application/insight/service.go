package insight

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/marcogerstmann/insight-processing-platform/internal/apperr"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/llm"
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
	repo ports.InsightRepository
	llm  *llm.Service
}

func NewService(repo ports.InsightRepository, llm *llm.Service) Service {
	return &service{
		repo: repo,
		llm:  llm,
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

	if s.llm == nil {
		slog.WarnContext(ctx, "no LLM service configured, skipping enrichment")
		return Result{Inserted: true}, nil
	}

	enrichment, err := s.llm.Enrich(ctx, insight.Text)
	if err != nil {
		slog.WarnContext(ctx, "enrichment failed, proceeding without enrichment", "err", err)
		return Result{Inserted: true}, nil
	}

	insight.Enrichment = &enrichment
	if err := s.repo.Update(ctx, insight); err != nil {
		return Result{}, err
	}

	return Result{Inserted: true}, nil
}

func (s *service) ListByTenantID(ctx context.Context, tenantID string) ([]domain.Insight, error) {
	return s.repo.ListByTenantID(ctx, tenantID)
}
