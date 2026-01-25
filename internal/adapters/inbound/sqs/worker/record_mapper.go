package worker

import (
	"github.com/aws/aws-lambda-go/events"
	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

func MapRecordToDomain(rec events.SQSMessage) (domain.IngestEvent, error) {
	dto, err := parseBody(rec.Body)
	if err != nil {
		return domain.IngestEvent{}, err
	}

	ev, err := MapMessageDTOToDomain(dto)
	if err != nil {
		return domain.IngestEvent{}, err
	}

	return ev, nil
}
