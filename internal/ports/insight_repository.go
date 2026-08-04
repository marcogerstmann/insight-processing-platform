package ports

import (
	"context"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

type InsightRepository interface {
	CreateIfAbsent(ctx context.Context, insight domain.Insight) (inserted bool, err error)
	Update(ctx context.Context, insight domain.Insight) error
	ListByTenantID(ctx context.Context, tenantID, tag string) ([]domain.Insight, error)
	ListByTag(ctx context.Context, tenantID, tag string) ([]domain.TagMembership, error)
	ListTags(ctx context.Context, tenantID string) ([]domain.TagSummary, error)
}
