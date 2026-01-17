package ingest

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidPayload = errors.New("invalid payload")
)

type IngestEvent struct {
	TenantID   string
	Source     string
	EventType  string
	ReceivedAt time.Time

	Highlight Highlight
}

type Highlight struct {
	ID            int64
	BookID        int64
	Text          string
	Note          string
	URL           string
	Tags          []string
	HighlightedAt time.Time
	UpdatedAt     time.Time
	Location      int
	LocationType  string
	Color         string
}

func MapReadwisePayload(p ReadwiseWebhookDTO, receivedAt time.Time) (IngestEvent, error) {
	if p.ID <= 0 {
		return IngestEvent{}, fmt.Errorf("%w: missing/invalid id", ErrInvalidPayload)
	}
	if strings.TrimSpace(p.EventType) == "" {
		return IngestEvent{}, fmt.Errorf("%w: missing event_type", ErrInvalidPayload)
	}
	if strings.TrimSpace(p.Text) == "" {
		return IngestEvent{}, fmt.Errorf("%w: missing highlight text", ErrInvalidPayload)
	}

	ev := IngestEvent{
		Source:     "readwise",
		EventType:  p.EventType,
		ReceivedAt: receivedAt,
		Highlight: Highlight{
			ID:            p.ID,
			BookID:        p.BookID,
			Text:          p.Text,
			Note:          p.Note,
			URL:           p.URL,
			Tags:          append([]string(nil), p.Tags...),
			HighlightedAt: p.HighlightedAt,
			UpdatedAt:     p.Updated,
			Location:      p.Location,
			LocationType:  p.LocationType,
			Color:         p.Color,
		},
	}

	return ev, nil
}
