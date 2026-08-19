package relationship

import "time"

type CreateRelationshipRequestDTO struct {
	ToInsightID string  `json:"to_insight_id"`
	Type        string  `json:"type"`
	Confidence  float64 `json:"confidence"`
	Rationale   string  `json:"rationale"`
}

type ResponseDTO struct {
	FromInsightID string    `json:"from_insight_id"`
	ToInsightID   string    `json:"to_insight_id"`
	Type          string    `json:"type"`
	Confidence    float64   `json:"confidence"`
	Rationale     string    `json:"rationale"`
	DiscoveredAt  time.Time `json:"discovered_at"`
}

type RelatedInsightDTO struct {
	InsightID  string  `json:"insight_id"`
	Text       string  `json:"text"`
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"`
	Rationale  string  `json:"rationale"`
}

type ListRelationshipsResponseDTO struct {
	InsightID string              `json:"insight_id"`
	Items     []RelatedInsightDTO `json:"items"`
}
