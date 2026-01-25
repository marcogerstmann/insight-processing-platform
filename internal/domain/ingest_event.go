package domain

import "time"

type IngestEvent struct {
	TenantID       string    `json:"tenantId"`
	Source         string    `json:"source"`
	EventType      string    `json:"eventType"`
	ReceivedAt     time.Time `json:"receivedAt"`
	IdempotencyKey string    `json:"idempotencyKey"`
	Highlight      Highlight `json:"highlight"`
}
