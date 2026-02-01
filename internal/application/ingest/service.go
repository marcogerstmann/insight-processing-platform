package ingest

import (
	"context"
	"encoding/json"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports/outbound/event"
)

type Service struct {
	Publisher event.EventPublisher
}

func NewService(p event.EventPublisher) *Service {
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

	msg := event.PublishMessage{
		Body: body,
		Attributes: map[string]string{
			"idempotencyKey": idempotencyKey,
			"tenantId":       tenantID,
		},
	}

	return s.Publisher.Publish(ctx, msg)
}
