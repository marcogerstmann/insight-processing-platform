package insight

type EnrichmentDTO struct {
	Tags []string `json:"tags"`
}

type ResponseDTO struct {
	ID         string         `json:"id"`
	Source     string         `json:"source"`
	Text       string         `json:"text"`
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
