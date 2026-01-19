package ingestapp

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mgerstmannsf/insight-processing-platform/internal/domain"
	"github.com/mgerstmannsf/insight-processing-platform/internal/ports/outbound"
)

type Service struct {
	Publisher outbound.EventPublisher
}

func NewService(p outbound.EventPublisher) *Service {
	return &Service{Publisher: p}
}

func (s *Service) EnqueueReadwise(ctx context.Context, ev domain.IngestEvent, receivedAt time.Time, tenantID string) error {
	ev.TenantID = tenantID

	idempotencyKey := buildIdempotencyKey(ev)

	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}

	msg := outbound.PublishMessage{
		Body: body,
		Attributes: map[string]string{
			"tenant_id":       tenantID,
			"event_type":      ev.EventType,
			"idempotency_key": idempotencyKey,
			"received_at":     receivedAt.UTC().Format(time.RFC3339),
		},
	}

	return s.Publisher.Publish(ctx, msg)
}
