package domain

import "time"

type IngestEvent struct {
	IdempotencyKey string    `json:"idempotency_key"`
	TenantID       string    `json:"tenant_id"`
	Source         string    `json:"source"`
	EventType      string    `json:"event_type"`
	ReceivedAt     time.Time `json:"received_at"`
	Highlight      Highlight `json:"highlight"`
}
