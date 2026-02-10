package outbound

import (
	"context"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

type InsightEnricher interface {
	Enrich(ctx context.Context, insight domain.Insight) (domain.Insight, error)
}
