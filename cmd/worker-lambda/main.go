package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	workersqs "github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/sqs/worker"
	anthropicAdapter "github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/anthropic"
	dynamoAdapters "github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/dynamodb"
	sqsAdapters "github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/sqs"
	ssmAdapters "github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/ssm"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/insight"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
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

	dlqPublisher, err := sqsAdapters.NewSQSDLQPublisher(ctx)
	if err != nil {
		log.Error("failed to create DLQ publisher", "err", err)
		os.Exit(1)
	}

	insightRepo := dynamoAdapters.NewInsightAdapter(dbclient, mustEnv("TABLE_NAME_INSIGHTS"))

	var enricher ports.InsightEnricher
	if apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")); apiKey != "" {
		enricher = anthropicAdapter.NewInsightEnricher(apiKey)
	} else if ssmPath := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY_SSM")); ssmPath != "" {
		secrets, err := ssmAdapters.NewSecretProvider(ctx)
		if err != nil {
			log.Error("failed to create SSM secret provider", "err", err)
			os.Exit(1)
		}
		apiKey, err := secrets.Get(ctx, ssmPath)
		if err != nil {
			log.Error("failed to load Anthropic API key from SSM", "path", ssmPath, "err", err)
			os.Exit(1)
		}
		enricher = anthropicAdapter.NewInsightEnricher(apiKey)
	}

	svc := insight.NewService(insightRepo, enricher)

	h := workersqs.NewHandler(svc, dlqPublisher, log)
	lambda.Start(h.Handle)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("missing env var: " + key)
	}
	return v
}
