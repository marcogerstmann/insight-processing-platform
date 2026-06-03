package insight

type InsightResponseDTO struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Text   string `json:"text"`
}

type ListInsightsResponseDTO struct {
	TenantID string               `json:"tenant_id"`
	Items    []InsightResponseDTO `json:"items"`
}
