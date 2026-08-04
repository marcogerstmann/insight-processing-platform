package domain

import "time"

type TagMembership struct {
	InsightID string
	CreatedAt time.Time
}

// TagSummary is a tenant's tag aggregated across its memberships: how many
// insights carry it and when the most recent one was tagged.
type TagSummary struct {
	Tag           string
	InsightCount  int
	LastInsightAt time.Time
}
