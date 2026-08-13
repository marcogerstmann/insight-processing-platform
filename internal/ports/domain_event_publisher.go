package ports

import (
	"context"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

// DomainEventPublisher announces a typed domain fact. Unlike EventPublisher
// (raw bytes on a work queue, for pipeline orchestration), this is for
// domain events that other bounded contexts subscribe to.
type DomainEventPublisher interface {
	Publish(ctx context.Context, event domain.DomainEvent) error
}
