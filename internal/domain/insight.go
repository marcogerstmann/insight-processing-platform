package domain

import "time"

type Insight struct {
	ID            string
	TenantID      string
	Source        string
	Text          string
	Notes         string
	Enrichment    *Enrichment
	HighlightedAt time.Time
}
