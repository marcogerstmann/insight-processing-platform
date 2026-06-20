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
	anthropicAdapter "github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/anthropic"
	dynamoAdapters "github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/dynamodb"
	sqsAdapters "github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/sqs"
	ssmAdapters "github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/ssm"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/insight"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/llm"
	"github.com/marcogerstmann/insight-processing-platform/internal/envutil"
	"github.com/marcogerstmann/insight-processing-platform/internal/logging"
)

func main() {
	log := logging.New(os.Stdout)
	slog.SetDefault(log)

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

	secretProvider, err := ssmAdapters.NewSecretProvider(ctx)
	if err != nil {
		log.Error("failed to create SSM secret provider", "err", err)
		os.Exit(1)
	}

	insightRepo := dynamoAdapters.NewInsightAdapter(dbclient, mustEnv("TABLE_NAME_INSIGHTS"))

	var llmService *llm.Service
	apiKey, err := envutil.ResolveSecret(ctx, "ANTHROPIC_API_KEY", secretProvider)
	if err != nil {
		log.Error("failed to resolve Anthropic API key", "err", err)
		os.Exit(1)
	}
	if apiKey != "" {
		llmService = llm.NewService(anthropicAdapter.NewClient(apiKey))
	}

	svc := insight.NewService(insightRepo, llmService)

	h := workersqs.NewHandler(svc, dlqPublisher)
	lambda.Start(h.Handle)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("missing env var: " + key)
	}
	return v
}
