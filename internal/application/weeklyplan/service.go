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

	// Get returns tenantID's plan with its actions' citations resolved to
	// the insights they refer to (PLAN 4/IPP-106).
	Get(ctx context.Context, tenantID, planID string) (domain.PlanDetail, error)

	// List returns tenantID's plans, newest first, without resolving
	// actions — GET .../weekly-plans is a list of plans to drill into, not
	// a feed of every action across all of them.
	List(ctx context.Context, tenantID string) ([]domain.WeeklyPlan, error)

	// Status returns just tenantID's plan's status — the Action Agent's
	// redelivery pre-check (PLAN 5/IPP-107), cheap on purpose: unlike Get
	// it never joins against insights, since the agent only needs to know
	// whether to skip regenerating.
	Status(ctx context.Context, tenantID, planID string) (domain.PlanStatus, error)

	SetReady(ctx context.Context, tenantID, planID string, actions []domain.Action) error
	SetFailed(ctx context.Context, tenantID, planID, reason string) error
}

type service struct {
	repo     ports.WeeklyPlanRepository
	insights ports.InsightRepository
	events   ports.DomainEventPublisher
}

func NewService(repo ports.WeeklyPlanRepository, insights ports.InsightRepository, events ports.DomainEventPublisher) Service {
	return &service{repo: repo, insights: insights, events: events}
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

// Get loads plan, then resolves each action's SupportingInsightIDs against
// the plan's own tag — the same bounded pool PLAN 2/3 drew the ids from in
// the first place, so one query covers every action's citations.
func (s *service) Get(ctx context.Context, tenantID, planID string) (domain.PlanDetail, error) {
	plan, err := s.repo.Get(ctx, tenantID, planID)
	if err != nil {
		return domain.PlanDetail{}, err
	}

	if len(plan.Actions) == 0 {
		return domain.PlanDetail{Plan: plan}, nil
	}

	taggedInsights, err := s.insights.ListByTenantID(ctx, tenantID, plan.Tag)
	if err != nil {
		return domain.PlanDetail{}, fmt.Errorf("load cited insights: %w", err)
	}
	textByID := make(map[string]string, len(taggedInsights))
	for _, insight := range taggedInsights {
		textByID[insight.ID] = insight.Text
	}

	actions := make([]domain.ResolvedAction, len(plan.Actions))
	for i, action := range plan.Actions {
		var supporting []domain.ResolvedInsight
		for _, id := range action.SupportingInsightIDs {
			text, ok := textByID[id]
			if !ok {
				// Cited insight deleted since the plan was generated —
				// same "skip the orphan" call listByTag already makes for
				// a stale tag membership.
				continue
			}
			supporting = append(supporting, domain.ResolvedInsight{InsightID: id, Text: text})
		}
		actions[i] = domain.ResolvedAction{
			Title:              action.Title,
			Why:                action.Why,
			SupportingInsights: supporting,
		}
	}

	return domain.PlanDetail{Plan: plan, Actions: actions}, nil
}

func (s *service) List(ctx context.Context, tenantID string) ([]domain.WeeklyPlan, error) {
	return s.repo.ListPlansByTenantID(ctx, tenantID)
}

func (s *service) Status(ctx context.Context, tenantID, planID string) (domain.PlanStatus, error) {
	plan, err := s.repo.Get(ctx, tenantID, planID)
	if err != nil {
		return "", err
	}
	return plan.Status, nil
}

func (s *service) SetReady(ctx context.Context, tenantID, planID string, actions []domain.Action) error {
	return s.repo.SetReady(ctx, tenantID, planID, actions)
}

func (s *service) SetFailed(ctx context.Context, tenantID, planID, reason string) error {
	return s.repo.SetFailed(ctx, tenantID, planID, reason)
}
