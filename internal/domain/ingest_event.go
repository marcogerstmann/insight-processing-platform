package domain

import "time"

type IngestEvent struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	Source     string    `json:"source"`
	EventType  string    `json:"event_type"`
	ReceivedAt time.Time `json:"received_at"`
	Highlight  Highlight `json:"highlight"`
}
