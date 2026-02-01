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

	return domain.IngestEvent{
		TenantID:       dto.TenantID,
		Source:         dto.Source,
		EventType:      dto.EventType,
		ReceivedAt:     dto.ReceivedAt,
		IdempotencyKey: dto.IdempotencyKey,
		Highlight: domain.Highlight{
			ID:   dto.Highlight.ID,
			Text: dto.Highlight.Text,
			Note: &dto.Highlight.Note,
			URL:  dto.Highlight.URL,
		},
	}, nil
}
