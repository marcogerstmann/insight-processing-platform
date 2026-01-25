package domain

import "time"

type IngestEvent struct {
	TenantID       string
	Source         string
	EventType      string
	ReceivedAt     time.Time
	IdempotencyKey string
	Highlight      Highlight `json:"highlight"`
}
