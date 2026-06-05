package memory

import (
	"context"
	"log/slog"
	"sync"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type InsightNoopAdapter struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

var _ ports.InsightRepository = (*InsightNoopAdapter)(nil)

func NewInsightNoopAdapter() *InsightNoopAdapter {
	return &InsightNoopAdapter{
		seen: make(map[string]struct{}),
	}
}

func (r *InsightNoopAdapter) CreateIfAbsent(_ context.Context, insight domain.Insight) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.seen[insight.ID]; exists {
		slog.Info("noop repo deduplicated insight",
			"tenantID", insight.TenantID,
			"id", insight.ID,
		)
		return false, nil
	}

	r.seen[insight.ID] = struct{}{}

	slog.Info("noop repo inserted insight",
		"id", insight.ID,
		"tenantID", insight.TenantID,
	)

	return true, nil
}

func (r *InsightNoopAdapter) Update(_ context.Context, insight domain.Insight) error {
	slog.Info("noop repo updated insight",
		"id", insight.ID,
		"tenantID", insight.TenantID,
	)
	return nil
}

func (r *InsightNoopAdapter) ListByTenantID(_ context.Context, tenantID string) ([]domain.Insight, error) {
	slog.Info("noop repo list insights", "tenantID", tenantID)
	return []domain.Insight{}, nil
}
