package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"

	workersqs "github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/sqs/worker"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/worker"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log := newLogger()

	bodyPath := filepath.Clean("./cmd/worker-local/event.body.json")

	bodyBytes, err := os.ReadFile(bodyPath)
	if err != nil {
		log.Error("failed to read body fixture", "path", bodyPath, "err", err)
		os.Exit(1)
	}

	var tmp map[string]any
	if err := json.Unmarshal(bodyBytes, &tmp); err != nil {
		log.Error("body fixture contains invalid JSON", "path", bodyPath, "err", err)
		os.Exit(1)
	}

	tenantID := strings.TrimSpace(getString(tmp, "tenantId"))
	idempotencyKey := strings.TrimSpace(getString(tmp, "idempotencyKey"))

	ev := events.SQSEvent{
		Records: []events.SQSMessage{
			{
				MessageId:     "local-test-message-1",
				ReceiptHandle: "local-receipt-handle",
				Body:          string(bodyBytes),
				Attributes: map[string]string{
					"ApproximateReceiveCount": "1",
				},
				MessageAttributes: map[string]events.SQSMessageAttribute{
					"tenantId": {
						StringValue: awsString(tenantID),
						DataType:    "String",
					},
					"idempotencyKey": {
						StringValue: awsString(idempotencyKey),
						DataType:    "String",
					},
				},
				EventSource:    "aws:sqs",
				EventSourceARN: "arn:aws:sqs:eu-central-1:123456789012:local-queue",
				AWSRegion:      "eu-central-1",
			},
		},
	}

	noopRepo := worker.NewNoopRepo(log)
	svc := worker.NewService(noopRepo)
	h := workersqs.NewHandler(svc, log)

	log.Info("invoking worker handler (local)",
		"fixture", bodyPath,
		"records", len(ev.Records),
	)

	if err := h.Handle(ctx, ev); err != nil {
		log.Error("worker handler returned error", "err", err)
		os.Exit(1)
	}

	log.Info("worker handler finished successfully")
}

func getString(m map[string]any, k string) string {
	v, ok := m[k]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func awsString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func newLogger() *slog.Logger {
	level := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL")))
	var slogLevel slog.Level

	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slogLevel,
	})
	return slog.New(h)
}
