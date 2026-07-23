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
	restauth "github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/auth"
	restinsight "github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/insight"
	dynamodbadapter "github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/dynamodb"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/insight"
	"github.com/marcogerstmann/insight-processing-platform/internal/logging"
)

var ginLambda *ginadapter.GinLambdaV2

func init() {
	logger := logging.New(os.Stdout)
	slog.SetDefault(logger)

	tableName := os.Getenv("TABLE_NAME_INSIGHTS")
	if tableName == "" {
		slog.Error("TABLE_NAME_INSIGHTS env var is required")
		os.Exit(1)
	}
	userPoolID := os.Getenv("COGNITO_USER_POOL_ID")
	if userPoolID == "" {
		slog.Error("COGNITO_USER_POOL_ID env var is required")
		os.Exit(1)
	}
	clientID := os.Getenv("COGNITO_CLIENT_ID")
	if clientID == "" {
		slog.Error("COGNITO_CLIENT_ID env var is required")
		os.Exit(1)
	}

	ctx := context.Background()
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		slog.Error("aws config failed", "err", err)
		os.Exit(1)
	}

	dynamoClient := awsdynamodb.NewFromConfig(awsCfg)
	insightAdapter := dynamodbadapter.NewInsightAdapter(dynamoClient, tableName)
	insightSvc := insight.NewService(insightAdapter, nil)
	insightHandler := restinsight.NewHandler(insightSvc)

	authValidator, err := restauth.NewCognitoValidator(ctx, awsCfg.Region, userPoolID, clientID)
	if err != nil {
		slog.Error("cognito validator setup failed", "err", err)
		os.Exit(1)
	}

	// CORS is handled by API Gateway (terraform/envs/dev/rest-api.tf), so no
	// allowed origins are passed here.
	ginLambda = ginadapter.NewV2(rest.NewRouter(insightHandler, authValidator, nil))
}

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return ginLambda.ProxyWithContext(ctx, req)
}

func main() {
	lambda.Start(handler)
}
