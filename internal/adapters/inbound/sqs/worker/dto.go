package worker

import "time"

type MessageDTO struct {
	TenantID       string       `json:"tenantId"`
	Source         string       `json:"source"`
	EventType      string       `json:"eventType"`
	ReceivedAt     time.Time    `json:"receivedAt"`
	IdempotencyKey string       `json:"idempotencyKey"`
	Highlight      HighlightDTO `json:"highlight"`
}

type HighlightDTO struct {
	ID   string  `json:"id"`
	Text string  `json:"text"`
	Note string  `json:"note"`
	URL  *string `json:"url"`
}
