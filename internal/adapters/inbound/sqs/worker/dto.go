package worker

import "time"

type messageDTO struct {
	TenantID   string       `json:"tenant_id"`
	Source     string       `json:"source"`
	EventType  string       `json:"event_type"`
	ReceivedAt time.Time    `json:"received_at"`
	ID         string       `json:"id"`
	Highlight  highlightDTO `json:"highlight"`
}

type highlightDTO struct {
	ID   string  `json:"id"`
	Text string  `json:"text"`
	Note string  `json:"note"`
	URL  *string `json:"url"`
}
