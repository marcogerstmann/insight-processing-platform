package worker

import (
	"fmt"
	"strings"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

type PermanentError struct{ Err error }

func (e PermanentError) Error() string { return e.Err.Error() }
func (e PermanentError) Unwrap() error { return e.Err }

func permanentf(format string, args ...any) error {
	return PermanentError{Err: fmt.Errorf(format, args...)}
}

func MapMessageDTOToDomain(dto MessageDTO) (domain.IngestEvent, error) {
	if strings.TrimSpace(dto.IdempotencyKey) == "" {
		return domain.IngestEvent{}, permanentf("missing idempotency key")
	}
	if strings.TrimSpace(dto.Source) == "" {
		return domain.IngestEvent{}, permanentf("missing source")
	}
	if strings.TrimSpace(dto.EventType) == "" {
		return domain.IngestEvent{}, permanentf("missing event type")
	}
	if dto.Highlight.ID <= 0 {
		return domain.IngestEvent{}, permanentf("invalid highlight id")
	}

	return domain.IngestEvent{
		TenantID:       dto.TenantID,
		Source:         dto.Source,
		EventType:      dto.EventType,
		ReceivedAt:     dto.ReceivedAt,
		IdempotencyKey: dto.IdempotencyKey,
		Highlight: domain.Highlight{
			ID:            dto.Highlight.ID,
			BookID:        dto.Highlight.BookID,
			Text:          dto.Highlight.Text,
			Note:          dto.Highlight.Note,
			URL:           dto.Highlight.URL,
			HighlightedAt: dto.Highlight.HighlightedAt,
			UpdatedAt:     dto.Highlight.UpdatedAt,
			Tags:          dto.Highlight.Tags,
			Location:      dto.Highlight.Location,
			LocationType:  dto.Highlight.LocationType,
			Color:         dto.Highlight.Color,
		},
	}, nil
}
