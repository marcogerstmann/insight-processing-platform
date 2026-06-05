package insight

import (
	"github.com/google/uuid"
	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

const sourceManual = "manual"

func newID() string {
	return uuid.New().String()
}

func mapInsightToDTO(i domain.Insight) InsightResponseDTO {
	return InsightResponseDTO{
		ID:      i.ID,
		Source:  i.Source,
		Text:    i.Text,
		Summary: i.Summary,
	}
}

func mapInsightsToDTO(tenantID string, insights []domain.Insight) ListInsightsResponseDTO {
	items := make([]InsightResponseDTO, len(insights))
	for idx, i := range insights {
		items[idx] = mapInsightToDTO(i)
	}
	return ListInsightsResponseDTO{TenantID: tenantID, Items: items}
}

func mapCreateRequestToDomain(tenantID string, req CreateInsightRequestDTO) domain.Insight {
	return domain.Insight{
		ID:       newID(),
		TenantID: tenantID,
		Source:   sourceManual,
		Text:     req.Text,
	}
}
