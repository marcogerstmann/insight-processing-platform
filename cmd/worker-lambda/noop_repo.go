package main

import (
	"context"
	"log/slog"
	"sync"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports/outbound"
)

// TODO delete once real persistence is implemented
type noopRepo struct {
	mu   sync.Mutex
	seen map[string]struct{}
	log  *slog.Logger
}

var _ outbound.InsightRepository = (*noopRepo)(nil)

func newNoopRepo(log *slog.Logger) *noopRepo {
	return &noopRepo{
		seen: make(map[string]struct{}),
		log:  log,
	}
}

func (r *noopRepo) PutIfAbsent(
	_ context.Context,
	insight domain.Insight,
) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.seen[insight.IdempotencyKey]; exists {
		r.log.Info(
			"noop repo deduplicated insight",
			"tenant_id", insight.TenantID,
			"highlight_id", insight.HighlightID,
			"idempotency_key", insight.IdempotencyKey,
		)
		return false, nil
	}

	r.seen[insight.IdempotencyKey] = struct{}{}

	r.log.Info(
		"noop repo inserted insight",
		"tenant_id", insight.TenantID,
		"highlight_id", insight.HighlightID,
		"idempotency_key", insight.IdempotencyKey,
	)

	return true, nil
}
