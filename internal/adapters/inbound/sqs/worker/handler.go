package worker

import (
	"context"
	"errors"
	"log/slog"

	"github.com/aws/aws-lambda-go/events"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/worker"
)

type Handler struct {
	svc *worker.Service
	log *slog.Logger
}

func NewHandler(svc *worker.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Handle(ctx context.Context, e events.SQSEvent) error {
	for _, rec := range e.Records {
		ev, err := MapRecordToDomain(rec)
		if err != nil {
			var perr PermanentError
			if errors.As(err, &perr) {
				h.log.WarnContext(ctx, "permanent error mapping sqs message (dropping)",
					"messageId", rec.MessageId,
					"err", err,
				)
				continue
			}

			h.log.ErrorContext(ctx, "unexpected error mapping sqs message (retrying)",
				"messageId", rec.MessageId,
				"err", err,
			)
			return err
		}

		res, err := h.svc.Process(ctx, ev)
		if err != nil {
			h.log.ErrorContext(ctx, "worker processing failed (retrying)",
				"messageId", rec.MessageId,
				"tenantId", ev.TenantID,
				"highlightId", ev.Highlight.ID,
				"err", err,
			)
			return err
		}

		h.log.InfoContext(ctx, "worker processed message",
			"messageId", rec.MessageId,
			"tenantId", ev.TenantID,
			"highlightId", ev.Highlight.ID,
			"inserted", res.Inserted,
		)
	}

	return nil
}
