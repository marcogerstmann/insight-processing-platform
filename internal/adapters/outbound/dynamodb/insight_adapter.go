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

type dynamoEnrichmentItem struct {
	Tags []string `dynamodbav:"tags"`
}

type dynamoInsightItem struct {
	PK         string                `dynamodbav:"pk"`
	SK         string                `dynamodbav:"sk"`
	TenantID   string                `dynamodbav:"tenant_id"`
	ID         string                `dynamodbav:"id"`
	Source     string                `dynamodbav:"source"`
	Text       string                `dynamodbav:"text"`
	Notes      string                `dynamodbav:"notes"`
	Enrichment *dynamoEnrichmentItem `dynamodbav:"enrichment,omitempty"`
	CreatedAt  time.Time             `dynamodbav:"created_at"`
	UpdatedAt  time.Time             `dynamodbav:"updated_at"`
}

func pk(tenantID string) string {
	return "TENANT#" + tenantID
}

func sk(id string) string {
	return "INSIGHT#" + id
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

func (r *InsightAdapter) CreateIfAbsent(ctx context.Context, insight domain.Insight) (bool, error) {
	now := r.now().UTC()

	item := dynamoInsightItem{
		PK:        pk(insight.TenantID),
		SK:        sk(insight.ID),
		ID:        insight.ID,
		TenantID:  insight.TenantID,
		Source:    insight.Source,
		Text:      insight.Text,
		Notes:     insight.Notes,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if insight.Enrichment != nil {
		item.Enrichment = &dynamoEnrichmentItem{
			Tags: insight.Enrichment.Tags,
		}
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

	if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
		// Item already exists, ignore to preserve idempotency.
		return false, nil
	}

	return false, err
}

func (r *InsightAdapter) ListByTenantID(ctx context.Context, tenantID string) ([]domain.Insight, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("#pk = :pk"),
		ExpressionAttributeNames: map[string]string{
			"#pk": "pk",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk(tenantID)},
		},
	})
	if err != nil {
		return nil, err
	}

	insights := make([]domain.Insight, 0, len(out.Items))
	for _, item := range out.Items {
		var dynItem dynamoInsightItem
		if err := attributevalue.UnmarshalMap(item, &dynItem); err != nil {
			return nil, err
		}
		insight := domain.Insight{
			ID:       dynItem.ID,
			TenantID: dynItem.TenantID,
			Source:   dynItem.Source,
			Text:     dynItem.Text,
			Notes:    dynItem.Notes,
		}

		if dynItem.Enrichment != nil {
			insight.Enrichment = &domain.Enrichment{
				Tags: dynItem.Enrichment.Tags,
			}
		}

		insights = append(insights, insight)
	}
	return insights, nil
}

func (r *InsightAdapter) Update(ctx context.Context, insight domain.Insight) error {
	key, err := attributevalue.MarshalMap(map[string]string{
		"pk": pk(insight.TenantID),
		"sk": sk(insight.ID),
	})
	if err != nil {
		return err
	}

	now := r.now().UTC()

	updateExpr := "SET #source = :source, #text = :text, #notes = :notes, #updated_at = :updated_at"
	exprNames := map[string]string{
		"#pk":         "pk",
		"#sk":         "sk",
		"#source":     "source",
		"#text":       "text",
		"#notes":      "notes",
		"#updated_at": "updated_at",
	}
	exprValues := map[string]types.AttributeValue{
		":source":     &types.AttributeValueMemberS{Value: insight.Source},
		":text":       &types.AttributeValueMemberS{Value: insight.Text},
		":notes":      &types.AttributeValueMemberS{Value: insight.Notes},
		":updated_at": &types.AttributeValueMemberS{Value: now.Format(time.RFC3339Nano)},
	}

	if insight.Enrichment != nil {
		enrichmentAV, err := attributevalue.MarshalMap(&dynamoEnrichmentItem{
			Tags: insight.Enrichment.Tags,
		})
		if err != nil {
			return fmt.Errorf("marshal enrichment: %w", err)
		}

		updateExpr += ", #enrichment = :enrichment"
		exprNames["#enrichment"] = "enrichment"
		exprValues[":enrichment"] = &types.AttributeValueMemberM{Value: enrichmentAV}
	}

	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.tableName),
		Key:                       key,
		ConditionExpression:       aws.String("attribute_exists(#pk) AND attribute_exists(#sk)"),
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeNames:  exprNames,
		ExpressionAttributeValues: exprValues,
	})

	if err == nil {
		return nil
	}

	if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
		return fmt.Errorf("insight not found for update (pk/sk missing) or condition failed")
	}

	return err
}
