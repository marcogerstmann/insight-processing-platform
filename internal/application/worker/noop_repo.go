package worker

import (
	"context"
	"log/slog"
	"sync"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports/outbound/persistence"
)

// TODO delete once real persistence is implemented
type NoopRepo struct {
	mu   sync.Mutex
	seen map[string]struct{}
	log  *slog.Logger
}

var _ persistence.InsightRepository = (*NoopRepo)(nil)

func NewNoopRepo(log *slog.Logger) *NoopRepo {
	return &NoopRepo{
		seen: make(map[string]struct{}),
		log:  log,
	}
}

func (r *NoopRepo) PutIfAbsent(
	_ context.Context,
	insight domain.Insight,
) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.seen[insight.IdempotencyKey]; exists {
		r.log.Info(
			"noop repo deduplicated insight",
			"tenantId", insight.TenantID,
			"idempotencyKey", insight.IdempotencyKey,
		)
		return false, nil
	}

	r.seen[insight.IdempotencyKey] = struct{}{}

	r.log.Info(
		"noop repo inserted insight",
		"tenantId", insight.TenantID,
		"idempotencyKey", insight.IdempotencyKey,
	)

	return true, nil
}
