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
	ID            int64      `json:"id"`
	BookID        int64      `json:"bookId"`
	Text          string     `json:"text"`
	Note          string     `json:"note"`
	URL           *string    `json:"url"`
	Tags          []string   `json:"tags"`
	HighlightedAt *time.Time `json:"highlightedAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	Location      int        `json:"location"`
	LocationType  string     `json:"locationType"`
	Color         string     `json:"color"`
}
