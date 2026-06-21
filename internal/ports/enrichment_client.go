package ports

import (
	"context"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

type EnrichmentClient interface {
	Enrich(ctx context.Context, text string) (domain.Enrichment, error)
}
