package domain

type Insight struct {
	TenantID       string
	IdempotencyKey string
	Source         string
	Text           string
}
