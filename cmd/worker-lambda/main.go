package main

import (
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	workersqs "github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/sqs/worker"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/worker"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	repo := worker.NewNoopRepo(log)
	svc := worker.NewService(repo)

	h := workersqs.NewHandler(svc, log)
	lambda.Start(h.Handle)
}
