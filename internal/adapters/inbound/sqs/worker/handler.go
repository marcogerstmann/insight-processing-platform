package worker

import (
	"context"
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
			h.log.ErrorContext(ctx, "failed to map sqs message (retrying)",
				"message_id", rec.MessageId,
				"err", err,
			)
			return err
		}

		res, err := h.svc.Process(ctx, ev)
		if err != nil {
			h.log.ErrorContext(ctx, "worker processing failed (retrying)",
				"message_id", rec.MessageId,
				"tenant_id", ev.TenantID,
				"highlight_id", ev.Highlight.ID,
				"err", err,
			)
			return err
		}

		h.log.InfoContext(ctx, "worker processed message",
			"message_id", rec.MessageId,
			"tenant_id", ev.TenantID,
			"highlight_id", ev.Highlight.ID,
			"inserted", res.Inserted,
		)
	}

	return nil
}
