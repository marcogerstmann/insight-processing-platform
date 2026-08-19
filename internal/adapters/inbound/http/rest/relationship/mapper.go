package relationship

import (
	"time"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

func mapCreateRequestToDomain(tenantID, fromInsightID string, req CreateRelationshipRequestDTO) domain.Relationship {
	return domain.Relationship{
		TenantID:      tenantID,
		FromInsightID: fromInsightID,
		ToInsightID:   req.ToInsightID,
		Type:          domain.RelationType(req.Type),
		Confidence:    req.Confidence,
		Rationale:     req.Rationale,
		DiscoveredAt:  time.Now().UTC(),
	}
}

func mapRelationshipToDTO(r domain.Relationship) ResponseDTO {
	return ResponseDTO{
		FromInsightID: r.FromInsightID,
		ToInsightID:   r.ToInsightID,
		Type:          string(r.Type),
		Confidence:    r.Confidence,
		Rationale:     r.Rationale,
		DiscoveredAt:  r.DiscoveredAt,
	}
}

func mapRelatedInsightsToDTO(insightID string, related []domain.RelatedInsight) ListRelationshipsResponseDTO {
	items := make([]RelatedInsightDTO, len(related))
	for idx, r := range related {
		items[idx] = RelatedInsightDTO{
			InsightID:  r.InsightID,
			Text:       r.Text,
			Type:       string(r.Type),
			Confidence: r.Confidence,
			Rationale:  r.Rationale,
		}
	}
	return ListRelationshipsResponseDTO{InsightID: insightID, Items: items}
}
