package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	ingestlambda "github.com/mgerstmannsf/insight-processing-platform/internal/adapters/inbound/apigw/ingest"
	"github.com/mgerstmannsf/insight-processing-platform/internal/adapters/outbound/sqs"
	"github.com/mgerstmannsf/insight-processing-platform/internal/application/ingest"
	"github.com/mgerstmannsf/insight-processing-platform/internal/application/tenant"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ctx := context.Background()

	publisher, err := sqs.NewSQSEventPublisher(ctx)
	if err != nil {
		log.Error("publisher init failed", "err", err)
		os.Exit(1)
	}

	ingestSvc := ingest.NewService(publisher)
	tenantResolver := tenant.NewResolver()

	h := ingestlambda.NewHandler(log, tenantResolver, ingestSvc)

	lambda.Start(h.Handle)
}
