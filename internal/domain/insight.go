package domain

import "time"

type Insight struct {
	TenantID       string
	IdempotencyKey string
	Source         string
	Text           string
	CreatedAt      time.Time
}
