package ingest

import (
	"context"
	"strings"
	"time"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type ImportResult struct {
	Fetched  int
	Enqueued int
}

// Importer bulk-imports a tenant's highlights from a source through the same
// Enqueue path a push webhook uses, so enrichment stays async and dedup is
// free. source and eventType are stamped onto every enqueued
// domain.IngestEvent; eventType must match the source's own webhook
// event_type for created highlights (e.g. apigw/readwise's webhookDTO) so
// that a highlight imported here and the same highlight delivered via that
// webhook hash to the same idempotency key (buildIdempotencyKey) and dedupe
// against each other at the DynamoDB layer (CreateIfAbsent).
type Importer struct {
	client    ports.HighlightSource
	svc       Service
	source    string
	eventType string
}

func NewImporter(client ports.HighlightSource, svc Service, source, eventType string) *Importer {
	return &Importer{client: client, svc: svc, source: source, eventType: eventType}
}

// Import fetches tenantID's highlights and enqueues each one.
// onlyFavorites, when true, drops non-favorited highlights before limit is
// applied, so "latest N favorites" means the N most recent favorites, not
// favorites among the N most recent highlights overall. limit <= 0 imports
// everything that survives filtering; otherwise only the limit most recently
// highlighted ones (FetchHighlights returns newest first).
func (im *Importer) Import(ctx context.Context, tenantID string, limit int, onlyFavorites bool) (ImportResult, error) {
	fetched, err := im.client.FetchHighlights(ctx)
	if err != nil {
		return ImportResult{}, err
	}

	highlights := make([]ports.SourceHighlight, 0, len(fetched))
	for _, h := range fetched {
		if onlyFavorites && !h.IsFavorite {
			continue
		}
		if strings.TrimSpace(h.Text) == "" {
			continue
		}
		highlights = append(highlights, h)
	}

	if limit > 0 && limit < len(highlights) {
		highlights = highlights[:limit]
	}

	receivedAt := time.Now().UTC()
	result := ImportResult{Fetched: len(highlights)}

	for _, h := range highlights {
		ev := domain.IngestEvent{
			TenantID:   tenantID,
			Source:     im.source,
			EventType:  im.eventType,
			ReceivedAt: receivedAt,
			Highlight: domain.Highlight{
				ID:            h.ID,
				Text:          h.Text,
				Note:          h.Note,
				URL:           h.URL,
				HighlightedAt: h.HighlightedAt,
			},
		}

		if err := im.svc.Enqueue(ctx, ev); err != nil {
			return result, err
		}
		result.Enqueued++
	}

	return result, nil
}
