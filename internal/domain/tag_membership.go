package domain

import "time"

type TagMembership struct {
	InsightID string
	CreatedAt time.Time
}

// TagSummary is a tenant's tag aggregated across its memberships: how many
// insights carry it, when the most recent one was tagged, and how relevant
// that usage is overall (see TagRelevanceScore).
type TagSummary struct {
	Tag             string
	InsightCount    int
	LastInsightAt   time.Time
	Score           float64
	ScoreComponents TagScoreComponents
}
