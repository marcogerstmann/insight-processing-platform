package domain

import "time"

type IngestEvent struct {
	IdempotencyKey string    `json:"idempotencyKey"`
	TenantID       string    `json:"tenantId"`
	Source         string    `json:"source"`
	EventType      string    `json:"eventType"`
	ReceivedAt     time.Time `json:"receivedAt"`
	Highlight      Highlight `json:"highlight"`
}
