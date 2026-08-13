package memory

import (
	"context"
	"log/slog"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type DomainEventNoopAdapter struct{}

var _ ports.DomainEventPublisher = (*DomainEventNoopAdapter)(nil)

func NewDomainEventNoopAdapter() *DomainEventNoopAdapter {
	return &DomainEventNoopAdapter{}
}

func (a *DomainEventNoopAdapter) Publish(_ context.Context, event domain.DomainEvent) error {
	slog.Info("noop domain event publisher: would publish",
		"event_id", event.EventID,
		"event_type", event.EventType,
		"tenant_id", event.TenantID,
	)
	return nil
}
