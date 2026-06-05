package insight

type InsightResponseDTO struct {
	ID      string `json:"id"`
	Source  string `json:"source"`
	Text    string `json:"text"`
	Summary string `json:"summary,omitempty"`
}

type ListInsightsResponseDTO struct {
	TenantID string               `json:"tenant_id"`
	Items    []InsightResponseDTO `json:"items"`
}

type CreateInsightRequestDTO struct {
	Text string `json:"text"`
}

type CreateInsightResponseDTO struct {
	Inserted bool               `json:"inserted"`
	Insight  InsightResponseDTO `json:"insight"`
}
