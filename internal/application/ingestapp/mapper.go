package ingestapp

import (
	"fmt"
	"strings"
	"time"

	"github.com/mgerstmannsf/insight-processing-platform/internal/adapters/inbound/lambda/ingest/dto"
	"github.com/mgerstmannsf/insight-processing-platform/internal/application/apperr"
	"github.com/mgerstmannsf/insight-processing-platform/internal/application/domain"
)

func MapReadwisePayload(p dto.ReadwiseWebhookDTO, receivedAt time.Time) (domain.IngestEvent, error) {
	if p.ID <= 0 {
		return domain.IngestEvent{}, apperr.E(apperr.ErrInvalidPayload, fmt.Errorf("missing/invalid id"))
	}
	if strings.TrimSpace(p.EventType) == "" {
		return domain.IngestEvent{}, apperr.E(apperr.ErrInvalidPayload, fmt.Errorf("missing event_type"))
	}
	if strings.TrimSpace(p.Text) == "" {
		return domain.IngestEvent{}, apperr.E(apperr.ErrInvalidPayload, fmt.Errorf("missing highlight text"))
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
