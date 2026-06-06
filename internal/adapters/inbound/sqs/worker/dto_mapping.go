package worker

import (
	"fmt"
	"strings"

	"github.com/marcogerstmann/insight-processing-platform/internal/apperr"
	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

func mapMessageDTOToDomain(dto messageDTO) (domain.IngestEvent, error) {
	if strings.TrimSpace(dto.Source) == "" {
		return domain.IngestEvent{}, apperr.PermanentError{Err: fmt.Errorf("missing source")}
	}
	if strings.TrimSpace(dto.EventType) == "" {
		return domain.IngestEvent{}, apperr.PermanentError{Err: fmt.Errorf("missing event type")}
	}
	if strings.TrimSpace(dto.Highlight.ID) == "" {
		return domain.IngestEvent{}, apperr.PermanentError{Err: fmt.Errorf("missing highlight id")}
	}

	return domain.IngestEvent{
		Source:     dto.Source,
		EventType:  dto.EventType,
		ReceivedAt: dto.ReceivedAt,
		Highlight: domain.Highlight{
			ID:   dto.Highlight.ID,
			Text: dto.Highlight.Text,
			Note: &dto.Highlight.Note,
			URL:  dto.Highlight.URL,
		},
	}, nil
}
