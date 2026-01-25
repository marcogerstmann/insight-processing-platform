package domain

import "time"

type Insight struct {
	CreatedAt      time.Time
	UpdatedAt      time.Time
	IdempotencyKey string
	TenantID       string
	HighlightID    int64
	Source         string
	EventType      string
	ReceivedAt     time.Time
	Text           string
	Note           string
	URL            string
	Tags           []string
}
