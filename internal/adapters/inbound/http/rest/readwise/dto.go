package readwise

// ImportRequestDTO is the POST /v1/readwise/import body.
// Token overrides the server-configured READWISE_API_TOKEN for this one
// request only (never persisted) — for tenants without one configured.
// Limit <= 0 (or omitted) imports every highlight; otherwise only the Limit
// most recently highlighted ones. OnlyFavorites, when true, imports only
// highlights favorited in Readwise.
type ImportRequestDTO struct {
	Token         string `json:"token,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	OnlyFavorites bool   `json:"only_favorites,omitempty"`
}

type ImportResponseDTO struct {
	Fetched  int `json:"fetched"`
	Enqueued int `json:"enqueued"`
}
