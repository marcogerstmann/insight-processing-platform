package ingest

import "time"

type ReadwiseWebhookDTO struct {
	ID            int64      `json:"id"`
	Text          string     `json:"text"`
	Note          string     `json:"note"`
	Location      int        `json:"location"`
	LocationType  string     `json:"locationType"`
	HighlightedAt *time.Time `json:"highlightedAt"`
	URL           *string    `json:"url"`
	Color         string     `json:"color"`
	Updated       time.Time  `json:"updated"`
	BookID        int64      `json:"bookId"`
	Tags          []string   `json:"tags"`
	EventType     string     `json:"eventType"`
	Secret        string     `json:"secret"`
}
