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

	eventPath := filepath.Clean("./cmd/worker-local/event.json")

	b, err := os.ReadFile(eventPath)
	if err != nil {
		log.Error("failed to read event file", "path", eventPath, "err", err)
		os.Exit(1)
	}

	var ev events.SQSEvent
	if err := json.Unmarshal(b, &ev); err != nil {
		log.Error("failed to unmarshal SQSEvent", "path", eventPath, "err", err)
		os.Exit(1)
	}

	noopRepo := worker.NewNoopRepo(log)
	svc := worker.NewService(noopRepo)

	h := workersqs.NewHandler(svc, log)

	log.Info("invoking worker handler (local)",
		"event_file", eventPath,
		"records", len(ev.Records),
	)

	if err := h.Handle(ctx, ev); err != nil {
		log.Error("worker handler returned error", "err", err)
		os.Exit(1)
	}

	log.Info("worker handler finished successfully")
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
