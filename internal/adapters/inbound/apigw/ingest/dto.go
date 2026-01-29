package ingest

import "time"

type ReadwiseWebhookDTO struct {
	ID            int64      `json:"id"`
	Text          string     `json:"text"`
	Note          string     `json:"note"`
	Location      int        `json:"location"`
	LocationType  string     `json:"location_type"`
	HighlightedAt *time.Time `json:"highlighted_at"`
	URL           *string    `json:"url"`
	Color         string     `json:"color"`
	Updated       time.Time  `json:"updated"`
	BookID        int64      `json:"book_id"`
	Tags          []string   `json:"tags"`
	EventType     string     `json:"event_type"`
	Secret        string     `json:"secret"`
}
