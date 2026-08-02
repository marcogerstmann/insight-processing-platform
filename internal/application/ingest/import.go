package ingest

import (
	"context"
	"strings"
	"time"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

// readwiseEventType must match the Readwise webhook's event_type for created
// highlights (see apigw/readwise's webhookDTO and dev/http/readwise-webhook.http)
// so that a highlight imported here and the same highlight delivered via the
// webhook hash to the same idempotency key (buildIdempotencyKey) and dedupe
// against each other at the DynamoDB layer (CreateIfAbsent).
const readwiseEventType = "readwise.highlight.created"

type ImportResult struct {
	Fetched  int
	Enqueued int
}

// Importer bulk-imports a tenant's Readwise highlights through the same
// Enqueue path the webhook uses, so enrichment stays async and dedup is free.
type Importer struct {
	client ports.ReadwiseClient
	svc    Service
}

func NewImporter(client ports.ReadwiseClient, svc Service) *Importer {
	return &Importer{client: client, svc: svc}
}

// Import fetches tenantID's Readwise highlights and enqueues each one.
// limit <= 0 imports everything; otherwise only the limit most recently
// highlighted ones (FetchHighlights returns newest first).
func (im *Importer) Import(ctx context.Context, tenantID, token string, limit int) (ImportResult, error) {
	highlights, err := im.client.FetchHighlights(ctx, token)
	if err != nil {
		return ImportResult{}, err
	}
	if limit > 0 && limit < len(highlights) {
		highlights = highlights[:limit]
	}

	receivedAt := time.Now().UTC()
	result := ImportResult{Fetched: len(highlights)}

	for _, h := range highlights {
		if strings.TrimSpace(h.Text) == "" {
			continue
		}

		ev := domain.IngestEvent{
			TenantID:   tenantID,
			Source:     "readwise",
			EventType:  readwiseEventType,
			ReceivedAt: receivedAt,
			Highlight: domain.Highlight{
				ID:   h.ID,
				Text: h.Text,
				Note: h.Note,
				URL:  h.URL,
			},
		}

		if err := im.svc.Enqueue(ctx, ev); err != nil {
			return result, err
		}
		result.Enqueued++
	}

	return result, nil
}
