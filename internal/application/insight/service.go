package insight

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

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
	ListByTenantID(ctx context.Context, tenantID, tag string) ([]domain.Insight, error)
	ListTags(ctx context.Context, tenantID string) ([]domain.TagSummary, error)
}

type service struct {
	repo   ports.InsightRepository
	llm    *llm.Service
	events ports.DomainEventPublisher
}

func NewService(repo ports.InsightRepository, llm *llm.Service, events ports.DomainEventPublisher) Service {
	return &service{
		repo:   repo,
		llm:    llm,
		events: events,
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

	// ponytail: a publish failure here (or after Update below) returns a
	// plain (transient) error so SQS redelivers — but on redelivery
	// CreateIfAbsent finds the record already there and short-circuits
	// above before reaching this publish. If publishing keeps failing
	// across every retry, the event is dropped despite the write having
	// succeeded. A "published" flag on the record (or an outbox table)
	// closes that gap if it ever bites; not worth it until it does.
	if err := s.publish(ctx, domain.NewInsightCreatedEvent(insight, time.Now())); err != nil {
		return Result{}, err
	}

	if s.llm == nil {
		slog.WarnContext(ctx, "no LLM service configured, skipping enrichment")
		return Result{Inserted: true}, nil
	}

	enrichmentInput := insight.Text
	if notes := strings.TrimSpace(insight.Notes); notes != "" {
		enrichmentInput += "\n\nNotes: " + notes
	}

	enrichment, err := s.llm.Enrich(ctx, enrichmentInput)
	if err != nil {
		slog.WarnContext(ctx, "enrichment failed, proceeding without enrichment", "err", err)
		return Result{Inserted: true}, nil
	}
	enrichment.Tags = domain.NormalizeTags(enrichment.Tags)

	insight.Enrichment = &enrichment
	if err := s.repo.Update(ctx, insight); err != nil {
		return Result{}, err
	}

	if err := s.publish(ctx, domain.NewInsightEnrichedEvent(insight, time.Now())); err != nil {
		return Result{}, err
	}

	return Result{Inserted: true}, nil
}

func (s *service) publish(ctx context.Context, event domain.DomainEvent) error {
	if err := s.events.Publish(ctx, event); err != nil {
		return fmt.Errorf("publish %s event: %w", event.EventType, err)
	}
	return nil
}

func (s *service) ListByTenantID(ctx context.Context, tenantID, tag string) ([]domain.Insight, error) {
	if tag == "" {
		return s.repo.ListByTenantID(ctx, tenantID, "")
	}

	normalized, ok := domain.NormalizeTag(tag)
	if !ok {
		return []domain.Insight{}, nil
	}
	return s.repo.ListByTenantID(ctx, tenantID, string(normalized))
}

func (s *service) ListTags(ctx context.Context, tenantID string) ([]domain.TagSummary, error) {
	return s.repo.ListTags(ctx, tenantID)
}
