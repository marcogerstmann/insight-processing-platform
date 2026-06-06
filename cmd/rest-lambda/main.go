package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"

	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest"
	restinsight "github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/insight"
	dynamodbadapter "github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/dynamodb"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/insight"
)

var ginLambda *ginadapter.GinLambdaV2

func init() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	tableName := os.Getenv("TABLE_NAME_INSIGHTS")
	if tableName == "" {
		slog.Error("TABLE_NAME_INSIGHTS env var is required")
		os.Exit(1)
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		slog.Error("aws config failed", "err", err)
		os.Exit(1)
	}

	dynamoClient := awsdynamodb.NewFromConfig(awsCfg)
	insightAdapter := dynamodbadapter.NewInsightAdapter(dynamoClient, tableName)
	insightSvc := insight.NewService(insightAdapter, nil)
	insightHandler := restinsight.NewHandler(insightSvc)

	ginLambda = ginadapter.NewV2(rest.NewRouter(insightHandler))
}

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return ginLambda.ProxyWithContext(ctx, req)
}

func main() {
	lambda.Start(handler)
}
