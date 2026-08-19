package relationship

import (
	"context"
	"fmt"
	"time"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type Service interface {
	Put(ctx context.Context, rel domain.Relationship) error
	ListByInsightID(ctx context.Context, tenantID, insightID string) ([]domain.RelatedInsight, error)
}

type service struct {
	repo   ports.RelationshipRepository
	events ports.DomainEventPublisher
}

func NewService(repo ports.RelationshipRepository, events ports.DomainEventPublisher) Service {
	return &service{repo: repo, events: events}
}

var _ Service = (*service)(nil)

// Put persists rel, then publishes KnowledgeUpdated (REL 5/IPP-101) so
// subscribers — the tag relevance score's density component today,
// something else tomorrow — learn the graph changed.
func (s *service) Put(ctx context.Context, rel domain.Relationship) error {
	if err := s.repo.Put(ctx, rel); err != nil {
		return err
	}

	event := domain.NewKnowledgeUpdatedEvent(rel, time.Now())
	if err := s.events.Publish(ctx, event); err != nil {
		return fmt.Errorf("publish %s event: %w", event.EventType, err)
	}
	return nil
}

func (s *service) ListByInsightID(ctx context.Context, tenantID, insightID string) ([]domain.RelatedInsight, error) {
	return s.repo.ListByInsightID(ctx, tenantID, insightID)
}
