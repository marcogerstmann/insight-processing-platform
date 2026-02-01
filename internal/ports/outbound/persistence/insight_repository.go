package persistence

import (
	"context"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

type InsightRepository interface {
	PutIfAbsent(ctx context.Context, insight domain.Insight) (inserted bool, err error)
}
