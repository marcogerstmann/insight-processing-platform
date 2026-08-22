package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

var _ ports.WeeklyPlanRepository = (*InsightAdapter)(nil)

type dynamoActionItem struct {
	Title                string   `dynamodbav:"title"`
	Why                  string   `dynamodbav:"why"`
	SupportingInsightIDs []string `dynamodbav:"supporting_insight_ids"`
}

type dynamoWeeklyPlanItem struct {
	PK            string             `dynamodbav:"pk"`
	SK            string             `dynamodbav:"sk"`
	TenantID      string             `dynamodbav:"tenant_id"`
	ID            string             `dynamodbav:"id"`
	Tag           string             `dynamodbav:"tag"`
	FocusSentence string             `dynamodbav:"focus_sentence"`
	Status        string             `dynamodbav:"status"`
	CreatedAt     time.Time          `dynamodbav:"created_at"`
	Actions       []dynamoActionItem `dynamodbav:"actions,omitempty"`
	FailureReason string             `dynamodbav:"failure_reason,omitempty"`
}

func (item dynamoWeeklyPlanItem) toDomain() domain.WeeklyPlan {
	actions := make([]domain.Action, len(item.Actions))
	for i, a := range item.Actions {
		actions[i] = domain.Action{
			Title:                a.Title,
			Why:                  a.Why,
			SupportingInsightIDs: a.SupportingInsightIDs,
		}
	}
	return domain.WeeklyPlan{
		ID:            item.ID,
		TenantID:      item.TenantID,
		Tag:           item.Tag,
		FocusSentence: item.FocusSentence,
		Status:        domain.PlanStatus(item.Status),
		CreatedAt:     item.CreatedAt,
		Actions:       actions,
		FailureReason: item.FailureReason,
	}
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

// Get loads one tenant's plan by id, or ErrPlanNotFound.
func (r *InsightAdapter) Get(ctx context.Context, tenantID, planID string) (domain.WeeklyPlan, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk(tenantID)},
			"sk": &types.AttributeValueMemberS{Value: planSK(planID)},
		},
	})
	if err != nil {
		return domain.WeeklyPlan{}, err
	}
	if out.Item == nil {
		return domain.WeeklyPlan{}, ports.ErrPlanNotFound
	}

	var item dynamoWeeklyPlanItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return domain.WeeklyPlan{}, err
	}
	return item.toDomain(), nil
}

// ListPlansByTenantID returns tenantID's plans, newest first. Sorted in Go
// rather than by sort key: sk is PLAN#<planID> (a UUID), which carries no
// chronological order the way TAG 4's insight sk does — fine at a personal
// tenant's usual handful of plans.
func (r *InsightAdapter) ListPlansByTenantID(ctx context.Context, tenantID string) ([]domain.WeeklyPlan, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("#pk = :pk AND begins_with(#sk, :skPrefix)"),
		ExpressionAttributeNames: map[string]string{
			"#pk": "pk",
			"#sk": "sk",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":       &types.AttributeValueMemberS{Value: pk(tenantID)},
			":skPrefix": &types.AttributeValueMemberS{Value: "PLAN#"},
		},
	})
	if err != nil {
		return nil, err
	}

	plans := make([]domain.WeeklyPlan, 0, len(out.Items))
	for _, raw := range out.Items {
		var item dynamoWeeklyPlanItem
		if err := attributevalue.UnmarshalMap(raw, &item); err != nil {
			return nil, err
		}
		plans = append(plans, item.toDomain())
	}
	sort.Slice(plans, func(i, j int) bool { return plans[i].CreatedAt.After(plans[j].CreatedAt) })
	return plans, nil
}

// SetReady conditionally moves a pending plan to ready with its drafted
// actions. The condition is the idempotency mechanism PLAN 5 (IPP-107)
// documents leaning on: a plan that's missing or no longer pending fails
// the same ConditionalCheckFailedException either way, so both of
// IPP-106's "unknown or already-ready" rejection cases collapse into one
// check with no extra read.
func (r *InsightAdapter) SetReady(ctx context.Context, tenantID, planID string, actions []domain.Action) error {
	items := make([]dynamoActionItem, len(actions))
	for i, a := range actions {
		items[i] = dynamoActionItem{
			Title:                a.Title,
			Why:                  a.Why,
			SupportingInsightIDs: a.SupportingInsightIDs,
		}
	}
	actionsAV, err := attributevalue.MarshalList(items)
	if err != nil {
		return err
	}

	return r.setResult(ctx, tenantID, planID, map[string]types.AttributeValue{
		":status":  &types.AttributeValueMemberS{Value: string(domain.PlanStatusReady)},
		":actions": &types.AttributeValueMemberL{Value: actionsAV},
	}, "SET #status = :status, #actions = :actions REMOVE #reason", map[string]string{
		"#actions": "actions",
		"#reason":  "failure_reason",
	})
}

// SetFailed conditionally moves a pending plan to failed with a
// human-readable reason. Same conditional-write shape as SetReady.
func (r *InsightAdapter) SetFailed(ctx context.Context, tenantID, planID, reason string) error {
	return r.setResult(ctx, tenantID, planID, map[string]types.AttributeValue{
		":status": &types.AttributeValueMemberS{Value: string(domain.PlanStatusFailed)},
		":reason": &types.AttributeValueMemberS{Value: reason},
	}, "SET #status = :status, #reason = :reason", map[string]string{
		"#reason": "failure_reason",
	})
}

func (r *InsightAdapter) setResult(
	ctx context.Context,
	tenantID, planID string,
	values map[string]types.AttributeValue,
	updateExpr string,
	extraNames map[string]string,
) error {
	names := map[string]string{"#pk": "pk", "#status": "status"}
	for k, v := range extraNames {
		names[k] = v
	}
	values[":pending"] = &types.AttributeValueMemberS{Value: string(domain.PlanStatusPending)}

	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk(tenantID)},
			"sk": &types.AttributeValueMemberS{Value: planSK(planID)},
		},
		ConditionExpression:       aws.String("attribute_exists(#pk) AND #status = :pending"),
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	if err != nil {
		if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
			return ports.ErrPlanNotPending
		}
		return err
	}
	return nil
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
