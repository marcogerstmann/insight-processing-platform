package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

// tagIndexName must match the GSI name declared in
// terraform/modules/dynamodb/main.tf (enable_tag_gsi = true).
const tagIndexName = "gsi1"

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

// dynamoTagMembershipItem lives in the same table/partition as its insight
// (pk = TENANT#<tenantID>, sk = TAG#<tag>#INSIGHT#<insightID>). gsi1pk/gsi1sk
// are only ever set on these items, never on dynamoInsightItem, which is what
// makes the GSI sparse: plain insights never appear in it.
type dynamoTagMembershipItem struct {
	PK        string    `dynamodbav:"pk"`
	SK        string    `dynamodbav:"sk"`
	GSI1PK    string    `dynamodbav:"gsi1pk"`
	GSI1SK    string    `dynamodbav:"gsi1sk"`
	InsightID string    `dynamodbav:"insight_id"`
	CreatedAt time.Time `dynamodbav:"created_at"`
}

func pk(tenantID string) string {
	return "TENANT#" + tenantID
}

func sk(id string) string {
	return "INSIGHT#" + id
}

func tagSK(tag, insightID string) string {
	return "TAG#" + tag + "#INSIGHT#" + insightID
}

type dynamoAPI interface {
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

type InsightAdapter struct {
	tableName string
	client    dynamoAPI
	now       func() time.Time
}

func NewInsightAdapter(client dynamoAPI, tableName string) *InsightAdapter {
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

func (r *InsightAdapter) ListByTenantID(ctx context.Context, tenantID, tag string) ([]domain.Insight, error) {
	if tag != "" {
		return r.listByTag(ctx, tenantID, tag)
	}

	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("#pk = :pk AND begins_with(#sk, :skPrefix)"),
		ExpressionAttributeNames: map[string]string{
			"#pk": "pk",
			"#sk": "sk",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":       &types.AttributeValueMemberS{Value: pk(tenantID)},
			":skPrefix": &types.AttributeValueMemberS{Value: "INSIGHT#"},
		},
	})
	if err != nil {
		return nil, err
	}

	insights := make([]domain.Insight, 0, len(out.Items))
	for _, item := range out.Items {
		insight, err := unmarshalInsight(item)
		if err != nil {
			return nil, err
		}
		insights = append(insights, insight)
	}
	return insights, nil
}

// listByTag resolves matching insight IDs via the sparse GSI, then fetches
// each full insight item. Fine at personal scale (a tag rarely spans more
// than a handful of insights); a BatchGetItem is the upgrade path if that
// ever stops being true.
func (r *InsightAdapter) listByTag(ctx context.Context, tenantID, tag string) ([]domain.Insight, error) {
	members, err := r.ListByTag(ctx, tenantID, tag)
	if err != nil {
		return nil, err
	}

	insights := make([]domain.Insight, 0, len(members))
	for _, m := range members {
		out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String(r.tableName),
			Key: map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberS{Value: pk(tenantID)},
				"sk": &types.AttributeValueMemberS{Value: sk(m.InsightID)},
			},
		})
		if err != nil {
			return nil, err
		}
		if out.Item == nil {
			// Orphaned membership (insight deleted after tagging); skip it.
			continue
		}
		insight, err := unmarshalInsight(out.Item)
		if err != nil {
			return nil, err
		}
		insights = append(insights, insight)
	}
	return insights, nil
}

func unmarshalInsight(item map[string]types.AttributeValue) (domain.Insight, error) {
	var dynItem dynamoInsightItem
	if err := attributevalue.UnmarshalMap(item, &dynItem); err != nil {
		return domain.Insight{}, err
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
	return insight, nil
}

// ListByTag returns the tag's membership items for a tenant via the sparse
// GSI, newest membership last (no read against the full insight item needed).
func (r *InsightAdapter) ListByTag(ctx context.Context, tenantID, tag string) ([]domain.TagMembership, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String(tagIndexName),
		KeyConditionExpression: aws.String("#gsi1pk = :pk AND begins_with(#gsi1sk, :skPrefix)"),
		ExpressionAttributeNames: map[string]string{
			"#gsi1pk": "gsi1pk",
			"#gsi1sk": "gsi1sk",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":       &types.AttributeValueMemberS{Value: pk(tenantID)},
			":skPrefix": &types.AttributeValueMemberS{Value: "TAG#" + tag + "#INSIGHT#"},
		},
	})
	if err != nil {
		return nil, err
	}

	memberships := make([]domain.TagMembership, 0, len(out.Items))
	for _, item := range out.Items {
		var dynItem dynamoTagMembershipItem
		if err := attributevalue.UnmarshalMap(item, &dynItem); err != nil {
			return nil, err
		}
		memberships = append(memberships, domain.TagMembership{
			InsightID: dynItem.InsightID,
			CreatedAt: dynItem.CreatedAt,
		})
	}
	return memberships, nil
}

// ListTags returns every tag in the tenant's partition aggregated with its
// insight count and most recent tagging time, sorted by count descending.
//
// Aggregates in Go over one query of the TAG# prefix, per the
// story's implementation notes. Fine at personal scale (a few hundred
// membership items); a materialized per-tag counter item is the upgrade
// path if this ever gets slow.
func (r *InsightAdapter) ListTags(ctx context.Context, tenantID string) ([]domain.TagSummary, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("#pk = :pk AND begins_with(#sk, :skPrefix)"),
		ExpressionAttributeNames: map[string]string{
			"#pk": "pk",
			"#sk": "sk",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":       &types.AttributeValueMemberS{Value: pk(tenantID)},
			":skPrefix": &types.AttributeValueMemberS{Value: "TAG#"},
		},
	})
	if err != nil {
		return nil, err
	}

	type aggregate struct {
		count  int
		lastAt time.Time
	}
	aggregates := make(map[string]*aggregate)
	var tagOrder []string

	for _, item := range out.Items {
		var dynItem dynamoTagMembershipItem
		if err := attributevalue.UnmarshalMap(item, &dynItem); err != nil {
			return nil, err
		}
		tag, ok := parseTagFromSK(dynItem.SK)
		if !ok {
			continue
		}

		a, exists := aggregates[tag]
		if !exists {
			a = &aggregate{}
			aggregates[tag] = a
			tagOrder = append(tagOrder, tag)
		}
		a.count++
		if dynItem.CreatedAt.After(a.lastAt) {
			a.lastAt = dynItem.CreatedAt
		}
	}

	summaries := make([]domain.TagSummary, 0, len(tagOrder))
	for _, tag := range tagOrder {
		a := aggregates[tag]
		summaries = append(summaries, domain.TagSummary{
			Tag:           tag,
			InsightCount:  a.count,
			LastInsightAt: a.lastAt,
		})
	}

	sort.SliceStable(summaries, func(i, j int) bool {
		return summaries[i].InsightCount > summaries[j].InsightCount
	})

	return summaries, nil
}

// parseTagFromSK extracts the tag from a membership item's sort key
// ("TAG#<tag>#INSIGHT#<insightID>"); tags are normalized (see
// domain.NormalizeTag) so they never contain "#".
func parseTagFromSK(sk string) (string, bool) {
	rest, ok := strings.CutPrefix(sk, "TAG#")
	if !ok {
		return "", false
	}
	tag, _, ok := strings.Cut(rest, "#INSIGHT#")
	if !ok {
		return "", false
	}
	return tag, true
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

	var oldTags []string
	if insight.Enrichment != nil {
		oldTags, err = r.currentTags(ctx, key)
		if err != nil {
			return fmt.Errorf("read current tags: %w", err)
		}
	}

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

	if err != nil {
		if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
			return fmt.Errorf("insight not found for update (pk/sk missing) or condition failed")
		}
		return err
	}

	if insight.Enrichment != nil {
		if err := r.syncTagMemberships(ctx, insight.TenantID, insight.ID, oldTags, insight.Enrichment.Tags, now); err != nil {
			return fmt.Errorf("sync tag memberships: %w", err)
		}
	}

	return nil
}

func (r *InsightAdapter) currentTags(ctx context.Context, key map[string]types.AttributeValue) ([]string, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       key,
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, nil
	}

	var current dynamoInsightItem
	if err := attributevalue.UnmarshalMap(out.Item, &current); err != nil {
		return nil, err
	}
	if current.Enrichment == nil {
		return nil, nil
	}
	return current.Enrichment.Tags, nil
}

// syncTagMemberships reconciles tag membership items with the newly enriched
// tag set: tags no longer present are deleted, newly added tags get a fresh
// membership item. Unchanged tags are left untouched so replaying the same
// enrichment doesn't reset created_at or write duplicates.
func (r *InsightAdapter) syncTagMemberships(ctx context.Context, tenantID, insightID string, oldTags, newTags []string, now time.Time) error {
	oldSet := toTagSet(oldTags)
	newSet := toTagSet(newTags)

	for tag := range oldSet {
		if newSet[tag] {
			continue
		}
		key, err := attributevalue.MarshalMap(map[string]string{
			"pk": pk(tenantID),
			"sk": tagSK(tag, insightID),
		})
		if err != nil {
			return err
		}
		if _, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(r.tableName),
			Key:       key,
		}); err != nil {
			return err
		}
	}

	for tag := range newSet {
		if oldSet[tag] {
			continue
		}
		item := dynamoTagMembershipItem{
			PK:        pk(tenantID),
			SK:        tagSK(tag, insightID),
			GSI1PK:    pk(tenantID),
			GSI1SK:    tagSK(tag, insightID),
			InsightID: insightID,
			CreatedAt: now,
		}
		av, err := attributevalue.MarshalMap(item)
		if err != nil {
			return err
		}
		if _, err := r.client.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(r.tableName),
			Item:      av,
		}); err != nil {
			return err
		}
	}

	return nil
}

func toTagSet(tags []string) map[string]bool {
	set := make(map[string]bool, len(tags))
	for _, t := range tags {
		set[t] = true
	}
	return set
}
