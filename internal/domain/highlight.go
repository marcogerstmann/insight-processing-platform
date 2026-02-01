package domain

type Highlight struct {
	ID   string  `json:"id"`
	Text string  `json:"text"`
	Note *string `json:"note"`
	URL  *string `json:"url"`
}
