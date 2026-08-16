// Package raindrop handles the scheduled poll trigger for Raindrop
// highlights (EventBridge Scheduler invokes the Lambda directly — see
// RAINDROP 6's terraform/envs/dev/raindrop.tf). This is Raindrop's
// equivalent of apigw/readwise's push webhook: the ingest edge stays
// event-driven, only the trigger transport differs.
package raindrop

import (
	"context"
	"log/slog"

	raindropclient "github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/raindrop"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/ingest"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

// Handler polls a single tenant's Raindrop highlights. There is no
// cursor/watermark: every run re-fetches up to Limit recent highlights and
// relies on buildIdempotencyKey to dedupe against previous runs and REST
// imports — a deliberate choice (see the epic's ADR), not an oversight.
type Handler struct {
	client   ports.HighlightSource
	svc      ingest.Service
	tenantID string
	limit    int
}

func NewHandler(client ports.HighlightSource, svc ingest.Service, tenantID string, limit int) *Handler {
	return &Handler{client: client, svc: svc, tenantID: tenantID, limit: limit}
}

// Poll fetches and enqueues tenantID's most recent Raindrop highlights, up to
// limit. Any upstream failure (auth, rate limit, timeout) is returned so the
// Lambda invocation is recorded as failed rather than silently no-op'ing.
func (h *Handler) Poll(ctx context.Context) error {
	importer := ingest.NewImporter(h.client, h.svc, "raindrop", raindropclient.EventType)

	result, err := importer.Import(ctx, h.tenantID, h.limit, false)
	if err != nil {
		slog.ErrorContext(ctx, "raindrop poll failed", "tenant_id", h.tenantID, "err", err)
		return err
	}

	slog.InfoContext(ctx, "raindrop poll complete",
		"tenant_id", h.tenantID, "fetched", result.Fetched, "enqueued", result.Enqueued)
	return nil
}
