package domain

import "time"

type TagMembership struct {
	InsightID string
	// CreatedAt is our own audit trail: when this membership was recorded.
	// Never used for relevance scoring — see HighlightedAt.
	CreatedAt time.Time
	// HighlightedAt is when the underlying insight was highlighted in its
	// source system (falling back to our own ingestion time for sources
	// with no such concept). This, not CreatedAt, is what tag relevance
	// scoring ranks on.
	HighlightedAt time.Time
}

// TagSummary is a tenant's tag aggregated across its memberships: how many
// insights carry it, when the most recent one was highlighted, and how
// relevant that usage is overall (see TagRelevanceScore).
type TagSummary struct {
	Tag             string
	InsightCount    int
	LastInsightAt   time.Time
	Score           float64
	ScoreComponents TagScoreComponents
}
