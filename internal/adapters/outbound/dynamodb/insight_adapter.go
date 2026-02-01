package dynamodb

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

type dynamoInsightItem struct {
	PK             string    `dynamodbav:"pk"`
	SK             string    `dynamodbav:"sk"`
	TenantID       string    `dynamodbav:"tenant_id"`
	IdempotencyKey string    `dynamodbav:"idempotency_key"`
	Source         string    `dynamodbav:"source"`
	Text           string    `dynamodbav:"text"`
	CreatedAt      time.Time `dynamodbav:"created_at"`
	InsertedAt     time.Time `dynamodbav:"inserted_at"`
}

func pk(tenantID string) string {
	return "TENANT#" + tenantID
}

func sk(idempotencyKey string) string {
	return "INSIGHT#" + idempotencyKey
}

type InsightAdapter struct {
	tableName string
	client    *dynamodb.Client
}

func NewInsightAdapter(client *dynamodb.Client, tableName string) *InsightAdapter {
	return &InsightAdapter{
		client:    client,
		tableName: tableName,
	}
}

func (r *InsightAdapter) PutIfAbsent(ctx context.Context, insight domain.Insight) (bool, error) {
	item := dynamoInsightItem{
		PK:             pk(insight.TenantID),
		SK:             sk(insight.IdempotencyKey),
		TenantID:       insight.TenantID,
		IdempotencyKey: insight.IdempotencyKey,
		Source:         insight.Source,
		Text:           insight.Text,
		CreatedAt:      insight.CreatedAt.UTC(),
		InsertedAt:     time.Now().UTC(),
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return false, err
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})

	if err == nil {
		return true, nil
	}

	var cfe *types.ConditionalCheckFailedException
	if errors.As(err, &cfe) {
		// Item already exists, ignore to preserve idempotency
		return false, nil
	}

	return false, err
}
