package insight

import (
	"github.com/google/uuid"
	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

const sourceManual = "manual"

func newID() string {
	return uuid.New().String()
}

func mapInsightToDTO(i domain.Insight) ResponseDTO {
	dto := ResponseDTO{
		ID:     i.ID,
		Source: i.Source,
		Text:   i.Text,
		Notes:  i.Notes,
	}

	if i.Enrichment != nil {
		dto.Enrichment = &EnrichmentDTO{
			Tags: i.Enrichment.Tags,
		}
	}

	return dto
}

func mapInsightsToDTO(tenantID string, insights []domain.Insight) ListInsightsResponseDTO {
	items := make([]ResponseDTO, len(insights))
	for idx, i := range insights {
		items[idx] = mapInsightToDTO(i)
	}
	return ListInsightsResponseDTO{TenantID: tenantID, Items: items}
}

func mapTagsToDTO(tenantID string, tags []domain.TagSummary) ListTagsResponseDTO {
	items := make([]TagResponseDTO, len(tags))
	for idx, t := range tags {
		items[idx] = TagResponseDTO{
			Tag:           t.Tag,
			InsightCount:  t.InsightCount,
			LastInsightAt: t.LastInsightAt,
		}
	}
	return ListTagsResponseDTO{TenantID: tenantID, Items: items}
}

func mapCreateRequestToDomain(tenantID string, req CreateInsightRequestDTO) domain.Insight {
	return domain.Insight{
		ID:       newID(),
		TenantID: tenantID,
		Source:   sourceManual,
		Text:     req.Text,
	}
}
