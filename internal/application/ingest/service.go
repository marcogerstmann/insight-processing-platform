package ingest

import (
	"context"
	"encoding/json"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports/outbound"
)

type Service struct {
	Publisher outbound.EventPublisher
}

func NewService(p outbound.EventPublisher) *Service {
	return &Service{Publisher: p}
}

func (s *Service) EnqueueReadwise(ctx context.Context, ev domain.IngestEvent, tenantID string) error {
	ev.TenantID = tenantID
	idempotencyKey := buildIdempotencyKey(ev)
	ev.IdempotencyKey = idempotencyKey

	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}

	msg := outbound.PublishMessage{
		Body: body,
		Attributes: map[string]string{
			"idempotency_key": idempotencyKey,
			"tenant_id":       tenantID,
		},
	}

	return s.Publisher.Publish(ctx, msg)
}
