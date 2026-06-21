package llm

import (
	"context"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type Service struct {
	client ports.EnrichmentClient
}

func NewService(client ports.EnrichmentClient) *Service {
	return &Service{client: client}
}

func (s *Service) Enrich(ctx context.Context, text string) (domain.Enrichment, error) {
	return s.client.Enrich(ctx, text)
}
