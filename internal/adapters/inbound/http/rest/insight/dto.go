package insight

import "time"

type EnrichmentDTO struct {
	Tags []string `json:"tags"`
}

type ResponseDTO struct {
	ID         string         `json:"id"`
	Source     string         `json:"source"`
	Text       string         `json:"text"`
	Notes      string         `json:"notes,omitempty"`
	Enrichment *EnrichmentDTO `json:"enrichment,omitempty"`
}

type ListInsightsResponseDTO struct {
	TenantID string        `json:"tenant_id"`
	Items    []ResponseDTO `json:"items"`
}

type CreateInsightRequestDTO struct {
	Text string `json:"text"`
}

type CreateInsightResponseDTO struct {
	Inserted bool        `json:"inserted"`
	Insight  ResponseDTO `json:"insight"`
}

type TagResponseDTO struct {
	Tag           string    `json:"tag"`
	InsightCount  int       `json:"insight_count"`
	LastInsightAt time.Time `json:"last_insight_at"`
}

type ListTagsResponseDTO struct {
	TenantID string           `json:"tenant_id"`
	Items    []TagResponseDTO `json:"items"`
}
