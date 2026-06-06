package worker

import (
	"context"
	"errors"
	"log/slog"

	"github.com/aws/aws-lambda-go/events"
	"github.com/marcogerstmann/insight-processing-platform/internal/apperr"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/insight"
	port "github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type Handler struct {
	svc insight.Service
	dlq port.DLQPublisher
}

func NewHandler(svc insight.Service, dlq port.DLQPublisher) *Handler {
	return &Handler{svc: svc, dlq: dlq}
}

func (h *Handler) Handle(ctx context.Context, e events.SQSEvent) error {
	for _, rec := range e.Records {
		ev, err := mapRecordToDomain(rec)
		if err != nil {
			if errors.As(err, &apperr.PermanentError{}) {
				h.routeToDLQ(ctx, rec, err)
				continue
			}
			slog.ErrorContext(ctx, "failed to map sqs message (transient, retrying)",
				"message_id", rec.MessageId,
				"err", err,
			)
			return err
		}

		i := mapIngestEventToInsight(ev)
		res, err := h.svc.Process(ctx, i)
		if err != nil {
			if errors.As(err, &apperr.PermanentError{}) {
				h.routeToDLQ(ctx, rec, err)
				continue
			}
			slog.ErrorContext(ctx, "worker processing failed (transient, retrying)",
				"message_id", rec.MessageId,
				"tenant_id", ev.TenantID,
				"highlight_id", ev.Highlight.ID,
				"err", err,
			)
			return err
		}

		slog.InfoContext(ctx, "worker processed message",
			"message_id", rec.MessageId,
			"tenant_id", ev.TenantID,
			"highlight_id", ev.Highlight.ID,
			"inserted", res.Inserted,
		)
	}

	return nil
}

func (h *Handler) routeToDLQ(ctx context.Context, rec events.SQSMessage, err error) {
	slog.ErrorContext(ctx, "permanent error, routed to DLQ",
		"message_id", rec.MessageId,
		"err", err,
	)
	if dlqErr := h.dlq.Send(ctx, rec, err); dlqErr != nil {
		slog.ErrorContext(ctx, "failed to send message to DLQ",
			"message_id", rec.MessageId,
			"err", dlqErr,
		)
	}
}
