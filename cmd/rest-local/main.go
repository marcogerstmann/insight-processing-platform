package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/joho/godotenv"

	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/rest"
	restinsight "github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/rest/insight"
	dynamodbadapter "github.com/marcogerstmann/insight-processing-platform/internal/adapters/outbound/dynamodb"
)

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	tableName := os.Getenv("TABLE_NAME_INSIGHTS")
	if tableName == "" {
		log.Fatal("TABLE_NAME_INSIGHTS env var is required")
	}

	ctx := context.Background()
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("aws config failed: %v", err)
	}

	dynamoClient := awsdynamodb.NewFromConfig(awsCfg)
	insightAdapter := dynamodbadapter.NewInsightAdapter(dynamoClient, tableName)

	insightHandler := restinsight.NewHandler(insightAdapter, logger)
	router := rest.NewRouter(insightHandler)

	addr := ":8081"
	log.Printf("REST server listening on http://localhost%s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
