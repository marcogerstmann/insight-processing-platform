package ports

import (
	"context"
	"errors"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

var (
	// ErrUnknownTag is returned by WeeklyPlanRepository.Create when plan.Tag
	// doesn't exist in plan.TenantID's partition.
	ErrUnknownTag = errors.New("unknown tag")

	// ErrPlanNotFound is returned by Get for a tenant/planID pair with no
	// stored plan.
	ErrPlanNotFound = errors.New("weekly plan not found")

	// ErrPlanNotPending is returned by SetReady/SetFailed when the plan
	// doesn't exist or has already left PlanStatusPending — IPP-106's
	// "writing a result to an unknown or already-ready plan is rejected",
	// covered by one conditional write rather than a lookup plus a check:
	// the same conditional write PLAN 5 (IPP-107) will lean on for
	// idempotency under redelivery.
	ErrPlanNotPending = errors.New("weekly plan is not pending")
)

type WeeklyPlanRepository interface {
	// Create persists plan, after checking plan.Tag exists for the tenant.
	Create(ctx context.Context, plan domain.WeeklyPlan) error

	// Get loads one tenant's plan by id, or ErrPlanNotFound.
	Get(ctx context.Context, tenantID, planID string) (domain.WeeklyPlan, error)

	// ListPlansByTenantID returns tenantID's plans, newest first. Named
	// distinctly from InsightRepository.ListByTenantID: *InsightAdapter
	// satisfies both interfaces, and Go methods share one namespace per type.
	ListPlansByTenantID(ctx context.Context, tenantID string) ([]domain.WeeklyPlan, error)

	// SetReady conditionally transitions a pending plan to ready with its
	// drafted actions, or returns ErrPlanNotPending.
	SetReady(ctx context.Context, tenantID, planID string, actions []domain.Action) error

	// SetFailed conditionally transitions a pending plan to failed with a
	// human-readable reason, or returns ErrPlanNotPending.
	SetFailed(ctx context.Context, tenantID, planID, reason string) error
}
