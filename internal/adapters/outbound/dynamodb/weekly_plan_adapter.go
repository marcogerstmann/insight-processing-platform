package dynamodb

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

var _ ports.WeeklyPlanRepository = (*InsightAdapter)(nil)

type dynamoWeeklyPlanItem struct {
	PK            string    `dynamodbav:"pk"`
	SK            string    `dynamodbav:"sk"`
	TenantID      string    `dynamodbav:"tenant_id"`
	ID            string    `dynamodbav:"id"`
	Tag           string    `dynamodbav:"tag"`
	FocusSentence string    `dynamodbav:"focus_sentence"`
	Status        string    `dynamodbav:"status"`
	CreatedAt     time.Time `dynamodbav:"created_at"`
}

func planSK(planID string) string {
	return "PLAN#" + planID
}

// Create persists plan (pk = TENANT#<id>, sk = PLAN#<planID> per IPP-103),
// after checking plan.Tag exists for the tenant — the same
// check-before-write shape as RelationshipRepository.Put's insight
// existence check.
func (r *InsightAdapter) Create(ctx context.Context, plan domain.WeeklyPlan) error {
	exists, err := r.tagExists(ctx, plan.TenantID, plan.Tag)
	if err != nil {
		return fmt.Errorf("check tag exists: %w", err)
	}
	if !exists {
		return ports.ErrUnknownTag
	}

	item := dynamoWeeklyPlanItem{
		PK:            pk(plan.TenantID),
		SK:            planSK(plan.ID),
		TenantID:      plan.TenantID,
		ID:            plan.ID,
		Tag:           plan.Tag,
		FocusSentence: plan.FocusSentence,
		Status:        string(plan.Status),
		CreatedAt:     plan.CreatedAt,
	}
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return err
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
	})
	return err
}

// tagExists reuses tagSK's "TAG#<tag>#INSIGHT#" prefix (an empty insightID
// yields exactly that prefix) to check whether any insight in the tenant
// carries tag, without a dedicated tag-existence index.
func (r *InsightAdapter) tagExists(ctx context.Context, tenantID, tag string) (bool, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("#pk = :pk AND begins_with(#sk, :skPrefix)"),
		ExpressionAttributeNames: map[string]string{
			"#pk": "pk",
			"#sk": "sk",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":       &types.AttributeValueMemberS{Value: pk(tenantID)},
			":skPrefix": &types.AttributeValueMemberS{Value: tagSK(tag, "")},
		},
	})
	if err != nil {
		return false, err
	}
	return len(out.Items) > 0, nil
}
