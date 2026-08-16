package ports

import (
	"context"
	"time"
)

// SourceHighlight is a highlight fetched from a HighlightSource for bulk import.
type SourceHighlight struct {
	ID            string
	Text          string
	Note          string
	URL           *string
	HighlightedAt time.Time
	IsFavorite    bool
}

// HighlightSource fetches a tenant's highlights from a source (Readwise,
// Raindrop, ...) for bulk import, as opposed to a push-based webhook
// (apigw/readwise).
type HighlightSource interface {
	// FetchHighlights returns all non-deleted highlights, newest (by
	// HighlightedAt) first.
	FetchHighlights(ctx context.Context) ([]SourceHighlight, error)
}
