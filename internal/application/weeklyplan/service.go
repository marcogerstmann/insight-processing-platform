package weeklyplan

import (
	"context"
	"fmt"
	"time"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type Service interface {
	Submit(ctx context.Context, plan domain.WeeklyPlan) error
}

type service struct {
	repo   ports.WeeklyPlanRepository
	events ports.DomainEventPublisher
}

func NewService(repo ports.WeeklyPlanRepository, events ports.DomainEventPublisher) Service {
	return &service{repo: repo, events: events}
}

var _ Service = (*service)(nil)

// Submit persists plan, then publishes WeeklyPlanRequested (PLAN 1/IPP-103)
// so the async planning work can pick it up.
func (s *service) Submit(ctx context.Context, plan domain.WeeklyPlan) error {
	if err := s.repo.Create(ctx, plan); err != nil {
		return err
	}

	event := domain.NewWeeklyPlanRequestedEvent(plan, time.Now())
	if err := s.events.Publish(ctx, event); err != nil {
		return fmt.Errorf("publish %s event: %w", event.EventType, err)
	}
	return nil
}
