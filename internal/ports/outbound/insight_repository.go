package outbound

import (
	"context"

	"github.com/mgerstmannsf/insight-processing-platform/internal/domain"
)

type InsightRepository interface {
	// PutIfAbsent must be idempotent: returns inserted=false when item already exists.
	PutIfAbsent(ctx context.Context, insight domain.Insight) (inserted bool, err error)
}
