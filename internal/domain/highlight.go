package domain

import "time"

type Highlight struct {
	ID   string  `json:"id"`
	Text string  `json:"text"`
	Note string  `json:"note"`
	URL  *string `json:"url"`
	// HighlightedAt is when the source system created this highlight, not
	// when we received it (see IngestEvent.ReceivedAt for that).
	HighlightedAt time.Time `json:"highlighted_at"`
}
