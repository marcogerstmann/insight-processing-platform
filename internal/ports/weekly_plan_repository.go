package ports

import (
	"context"
	"errors"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

// ErrUnknownTag is returned by WeeklyPlanRepository.Create when plan.Tag
// doesn't exist in plan.TenantID's partition.
var ErrUnknownTag = errors.New("unknown tag")

type WeeklyPlanRepository interface {
	// Create persists plan, after checking plan.Tag exists for the tenant.
	Create(ctx context.Context, plan domain.WeeklyPlan) error
}
