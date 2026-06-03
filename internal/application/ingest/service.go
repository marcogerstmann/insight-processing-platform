package ingest

import (
	"context"
	"encoding/json"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type Service interface {
	Enqueue(ctx context.Context, ev domain.IngestEvent, tenantID string) error
}

type service struct {
	publisher ports.EventPublisher
}

var _ Service = (*service)(nil)

func NewService(p ports.EventPublisher) Service {
	return &service{publisher: p}
}

func (s *service) Enqueue(ctx context.Context, ev domain.IngestEvent, tenantID string) error {
	ev.TenantID = tenantID
	id := buildIdempotencyKey(ev)
	ev.ID = id

	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}

	msg := ports.PublishMessage{
		Body: body,
		Attributes: map[string]string{
			"idempotency_key": id,
			"tenant_id":       tenantID,
		},
	}

	return s.publisher.Publish(ctx, msg)
}
