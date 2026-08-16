package raindrop

// ImportRequestDTO is the POST /v1/raindrop/import body.
// Token overrides the server-configured RAINDROP_API_TOKEN for this one
// request only (never persisted) — for tenants without one configured.
// Limit <= 0 (or omitted) imports every highlight. There is no
// OnlyFavorites field: Raindrop has no favourites concept.
type ImportRequestDTO struct {
	Token string `json:"token,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type ImportResponseDTO struct {
	Fetched  int `json:"fetched"`
	Enqueued int `json:"enqueued"`
}
