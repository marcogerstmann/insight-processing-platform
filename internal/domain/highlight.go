package domain

import "time"

type Highlight struct {
	ID            int64     `json:"id"`
	BookID        int64     `json:"book_id"`
	Text          string    `json:"text"`
	Note          string    `json:"note"`
	URL           string    `json:"url"`
	Tags          []string  `json:"tags"`
	HighlightedAt time.Time `json:"highlighted_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Location      int       `json:"location"`
	LocationType  string    `json:"location_type"`
	Color         string    `json:"color"`
}
