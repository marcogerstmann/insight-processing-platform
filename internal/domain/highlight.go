package domain

import "time"

type Highlight struct {
	ID            int64      `json:"id"`
	BookID        int64      `json:"bookId"`
	Text          string     `json:"text"`
	Note          string     `json:"note"`
	URL           *string    `json:"url"`
	Tags          []string   `json:"tags"`
	HighlightedAt *time.Time `json:"highlightedAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	Location      int        `json:"location"`
	LocationType  string     `json:"locationType"`
	Color         string     `json:"color"`
}
