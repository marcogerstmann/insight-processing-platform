package memory

import (
	"context"
	"log/slog"

	"github.com/aws/aws-lambda-go/events"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type DLQNoopAdapter struct {
	log *slog.Logger
}

var _ ports.DLQPublisher = (*DLQNoopAdapter)(nil)

func NewDLQNoopAdapter(log *slog.Logger) *DLQNoopAdapter {
	return &DLQNoopAdapter{log: log}
}

func (d *DLQNoopAdapter) Send(_ context.Context, record events.SQSMessage, reason error) error {
	d.log.Info("noop dlq: would route message to DLQ",
		"message_id", record.MessageId,
		"failure_reason", reason.Error(),
	)
	return nil
}
