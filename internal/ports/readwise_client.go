package ports

import (
	"context"
	"time"
)

// ReadwiseHighlight is a highlight fetched from the Readwise API for bulk import.
type ReadwiseHighlight struct {
	ID            string
	Text          string
	Note          string
	URL           *string
	HighlightedAt time.Time
}

// ReadwiseClient fetches a tenant's highlights from Readwise for bulk import,
// as opposed to the push-based webhook (apigw/readwise).
type ReadwiseClient interface {
	// FetchHighlights returns all non-deleted highlights for token, newest
	// (by HighlightedAt) first.
	FetchHighlights(ctx context.Context, token string) ([]ReadwiseHighlight, error)
}
