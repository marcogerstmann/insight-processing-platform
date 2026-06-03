package insight

import "github.com/marcogerstmann/insight-processing-platform/internal/domain"

func mapInsightToDTO(i domain.Insight) InsightResponseDTO {
	return InsightResponseDTO{
		IdempotencyKey: i.IdempotencyKey,
		Source:         i.Source,
		Text:           i.Text,
	}
}

func mapInsightsToDTO(tenantID string, insights []domain.Insight) ListInsightsResponseDTO {
	items := make([]InsightResponseDTO, len(insights))
	for idx, i := range insights {
		items[idx] = mapInsightToDTO(i)
	}
	return ListInsightsResponseDTO{TenantID: tenantID, Items: items}
}
