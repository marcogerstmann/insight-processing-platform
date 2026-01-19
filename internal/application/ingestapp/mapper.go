package ingestapp

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mgerstmannsf/insight-processing-platform/internal/adapters/inbound/lambda/ingest/dto"
	"github.com/mgerstmannsf/insight-processing-platform/internal/application/domain"
)

var (
	ErrInvalidPayload = errors.New("invalid payload")
)

func MapReadwisePayload(p dto.ReadwiseWebhookDTO, receivedAt time.Time) (domain.IngestEvent, error) {
	if p.ID <= 0 {
		return domain.IngestEvent{}, fmt.Errorf("%w: missing/invalid id", ErrInvalidPayload)
	}
	if strings.TrimSpace(p.EventType) == "" {
		return domain.IngestEvent{}, fmt.Errorf("%w: missing event_type", ErrInvalidPayload)
	}
	if strings.TrimSpace(p.Text) == "" {
		return domain.IngestEvent{}, fmt.Errorf("%w: missing highlight text", ErrInvalidPayload)
	}

	var tags []string
	if len(p.Tags) > 0 {
		tags = append([]string(nil), p.Tags...)
	} else {
		tags = []string{}
	}

	ev := domain.IngestEvent{
		Source:     "readwise",
		EventType:  p.EventType,
		ReceivedAt: receivedAt,
		Highlight: domain.Highlight{
			ID:            p.ID,
			BookID:        p.BookID,
			Text:          p.Text,
			Note:          p.Note,
			URL:           p.URL,
			Tags:          tags,
			HighlightedAt: p.HighlightedAt,
			UpdatedAt:     p.Updated,
			Location:      p.Location,
			LocationType:  p.LocationType,
			Color:         p.Color,
		},
	}

	return ev, nil
}
