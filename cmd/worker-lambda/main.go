package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	workersqs "github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/sqs/worker"
	dynamoAdapters "github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/dynamodb"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/worker"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Error("failed to load aws config", "err", err)
		os.Exit(1)
	}
	dbclient := dynamodb.NewFromConfig(awsCfg)

	insightRepo := dynamoAdapters.NewInsightAdapter(dbclient, mustEnv("TABLE_NAME_INSIGHTS"))
	svc := worker.NewService(insightRepo)

	h := workersqs.NewHandler(svc, log)
	lambda.Start(h.Handle)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("missing env var: " + key)
	}
	return v
}
