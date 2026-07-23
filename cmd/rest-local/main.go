package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/joho/godotenv"

	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest"
	restauth "github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/auth"
	restinsight "github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/insight"
	dynamodbadapter "github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/dynamodb"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/insight"
)

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	tableName := os.Getenv("TABLE_NAME_INSIGHTS")
	if tableName == "" {
		log.Fatal("TABLE_NAME_INSIGHTS env var is required")
	}
	userPoolID := os.Getenv("COGNITO_USER_POOL_ID")
	if userPoolID == "" {
		log.Fatal("COGNITO_USER_POOL_ID env var is required")
	}
	clientID := os.Getenv("COGNITO_CLIENT_ID")
	if clientID == "" {
		log.Fatal("COGNITO_CLIENT_ID env var is required")
	}

	ctx := context.Background()
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("aws config failed: %v", err)
	}

	dynamoClient := awsdynamodb.NewFromConfig(awsCfg)
	insightAdapter := dynamodbadapter.NewInsightAdapter(dynamoClient, tableName)
	// Enrichment is async and belongs to the worker path only — REST returns fast.
	insightSvc := insight.NewService(insightAdapter, nil)

	authValidator, err := restauth.NewCognitoValidator(ctx, awsCfg.Region, userPoolID, clientID)
	if err != nil {
		log.Fatalf("cognito validator setup failed: %v", err)
	}

	insightHandler := restinsight.NewHandler(insightSvc)
	// Allow the web app's Vite dev server to call this local API from the
	// browser. In AWS this is API Gateway's job; locally the Go server must do it.
	router := rest.NewRouter(insightHandler, authValidator, []string{"http://localhost:5173"})

	addr := ":8081"
	log.Printf("REST server listening on http://localhost%s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
