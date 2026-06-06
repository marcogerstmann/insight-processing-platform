package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/apigw/readwise"
	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/sqs"
	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/ssm"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/ingest"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/tenant"
	"github.com/marcogerstmann/insight-processing-platform/internal/logging"
)

func main() {
	log := logging.New(os.Stdout)
	slog.SetDefault(log)

	ctx := context.Background()

	publisher, err := sqs.NewSQSEventPublisher(ctx)
	if err != nil {
		log.Error("publisher init failed", "err", err)
		os.Exit(1)
	}

	secretProvider, err := ssm.NewSecretProvider(ctx)
	if err != nil {
		log.Error("ssm provider init failed", "err", err)
		os.Exit(1)
	}

	ingestSvc := ingest.NewService(publisher)
	tenantResolver := tenant.NewResolver()

	h := readwise.NewHandler(secretProvider, tenantResolver, ingestSvc)

	lambda.Start(h.Handle)
}
