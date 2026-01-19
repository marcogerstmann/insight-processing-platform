package ingest

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mgerstmannsf/insight-processing-platform/internal/application/apperr"
	"github.com/mgerstmannsf/insight-processing-platform/internal/domain"
)

func mapReadwiseDTOToDomain(p ReadwiseWebhookDTO, receivedAt time.Time, tenantID string) (domain.IngestEvent, error) {
	if p.ID <= 0 {
		return domain.IngestEvent{}, apperr.E(apperr.ErrInvalidPayload, errors.New("missing/invalid highlight id"))
	}
	if strings.TrimSpace(p.EventType) == "" {
		return domain.IngestEvent{}, apperr.E(apperr.ErrInvalidPayload, errors.New("missing event_type"))
	}
	if strings.TrimSpace(p.Text) == "" {
		return domain.IngestEvent{}, apperr.E(apperr.ErrInvalidPayload, fmt.Errorf("empty highlight text (id=%d)", p.ID))
	}

	ev := domain.IngestEvent{
		TenantID:   tenantID,
		Source:     "readwise",
		EventType:  p.EventType,
		ReceivedAt: receivedAt.UTC(),
		Highlight: domain.Highlight{
			ID:            p.ID,
			BookID:        p.BookID,
			Text:          p.Text,
			Note:          p.Note,
			URL:           p.URL,
			Tags:          p.Tags,
			HighlightedAt: p.HighlightedAt,
			UpdatedAt:     p.Updated,
			Location:      p.Location,
			LocationType:  p.LocationType,
			Color:         p.Color,
		},
	}

	return ev, nil
}
