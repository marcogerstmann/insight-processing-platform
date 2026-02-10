package dynamodb

import (
	"context"
	"errors"
	"fmt"
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
	UpdatedAt      time.Time `dynamodbav:"updated_at"`
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
	now       func() time.Time
}

func NewInsightAdapter(client *dynamodb.Client, tableName string) *InsightAdapter {
	return &InsightAdapter{
		client:    client,
		tableName: tableName,
		now:       time.Now,
	}
}

func (r *InsightAdapter) PutIfAbsent(ctx context.Context, insight domain.Insight) (bool, error) {
	now := r.now().UTC()

	item := dynamoInsightItem{
		PK:             pk(insight.TenantID),
		SK:             sk(insight.IdempotencyKey),
		TenantID:       insight.TenantID,
		IdempotencyKey: insight.IdempotencyKey,
		Source:         insight.Source,
		Text:           insight.Text,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return false, err
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,

		// No duplicates per (pk, sk)
		ConditionExpression: aws.String("attribute_not_exists(#pk)"),
		ExpressionAttributeNames: map[string]string{
			"#pk": "pk",
		},
	})

	if err == nil {
		return true, nil
	}

	var cfe *types.ConditionalCheckFailedException
	if errors.As(err, &cfe) {
		// Item already exists, ignore to preserve idempotency.
		return false, nil
	}

	return false, err
}

func (r *InsightAdapter) Update(ctx context.Context, insight domain.Insight) error {
	key, err := attributevalue.MarshalMap(map[string]string{
		"pk": pk(insight.TenantID),
		"sk": sk(insight.IdempotencyKey),
	})
	if err != nil {
		return err
	}

	now := r.now().UTC()

	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key:       key,

		// IMPORTANT: no upsert. If the item doesn't exist, fail.
		ConditionExpression: aws.String("attribute_exists(#pk) AND attribute_exists(#sk)"),

		UpdateExpression: aws.String(
			"SET #source = :source, #text = :text, #updated_at = :updated_at",
		),

		ExpressionAttributeNames: map[string]string{
			"#pk":         "pk",
			"#sk":         "sk",
			"#source":     "source",
			"#text":       "text",
			"#updated_at": "updated_at",
		},

		ExpressionAttributeValues: map[string]types.AttributeValue{
			":source":     &types.AttributeValueMemberS{Value: insight.Source},
			":text":       &types.AttributeValueMemberS{Value: insight.Text},
			":updated_at": &types.AttributeValueMemberS{Value: now.Format(time.RFC3339Nano)},
		},
	})

	if err == nil {
		return nil
	}

	var cfe *types.ConditionalCheckFailedException
	if errors.As(err, &cfe) {
		return fmt.Errorf("insight not found for update (pk/sk missing) or condition failed")
	}

	return err
}
