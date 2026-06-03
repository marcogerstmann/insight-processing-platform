package insight

type InsightResponseDTO struct {
	IdempotencyKey string `json:"idempotency_key"`
	Source         string `json:"source"`
	Text           string `json:"text"`
}

type ListInsightsResponseDTO struct {
	TenantID string               `json:"tenant_id"`
	Items    []InsightResponseDTO `json:"items"`
}
