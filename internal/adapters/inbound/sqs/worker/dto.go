package worker

import "time"

type MessageDTO struct {
	TenantID       string       `json:"tenant_id"`
	Source         string       `json:"source"`
	EventType      string       `json:"event_type"`
	ReceivedAt     time.Time    `json:"received_at"`
	IdempotencyKey string       `json:"idempotency_key"`
	Highlight      HighlightDTO `json:"highlight"`
}

type HighlightDTO struct {
	ID   string  `json:"id"`
	Text string  `json:"text"`
	Note string  `json:"note"`
	URL  *string `json:"url"`
}
