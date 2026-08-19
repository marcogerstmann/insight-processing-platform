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

var _ ports.RelationshipRepository = (*InsightAdapter)(nil)

// dynamoRelationshipItem is written twice per edge — once under
// relSK(from, to) and once under relSK(to, from) — so "related insights"
// for either insight ID is a single begins_with(sk, "REL#<id>#") query.
// Both copies keep the edge's original direction in
// FromInsightID/ToInsightID; only the sort key differs.
type dynamoRelationshipItem struct {
	PK            string    `dynamodbav:"pk"`
	SK            string    `dynamodbav:"sk"`
	TenantID      string    `dynamodbav:"tenant_id"`
	FromInsightID string    `dynamodbav:"from_insight_id"`
	ToInsightID   string    `dynamodbav:"to_insight_id"`
	Type          string    `dynamodbav:"type"`
	Confidence    float64   `dynamodbav:"confidence"`
	Rationale     string    `dynamodbav:"rationale"`
	DiscoveredAt  time.Time `dynamodbav:"discovered_at"`
}

func relSK(fromInsightID, toInsightID string) string {
	return "REL#" + fromInsightID + "#" + toInsightID
}

// Put persists rel as two adjacency items sharing the tenant's partition,
// after checking both insights exist. Both PutItems are unconditional
// (deterministic sk = upsert), which is what makes a re-post of the same
// edge idempotent rather than a duplicate.
//
// TRADE-OFF: the two PutItems aren't transactional, so a failure between
// them can leave one direction indexed and not the other. Upgrade to
// TransactWriteItems if that inconsistency ever surfaces in practice.
func (r *InsightAdapter) Put(ctx context.Context, rel domain.Relationship) error {
	fromExists, err := r.insightExists(ctx, rel.TenantID, rel.FromInsightID)
	if err != nil {
		return fmt.Errorf("check from insight exists: %w", err)
	}
	toExists, err := r.insightExists(ctx, rel.TenantID, rel.ToInsightID)
	if err != nil {
		return fmt.Errorf("check to insight exists: %w", err)
	}
	if !fromExists || !toExists {
		return ports.ErrInsightNotFound
	}

	item := dynamoRelationshipItem{
		PK:            pk(rel.TenantID),
		TenantID:      rel.TenantID,
		FromInsightID: rel.FromInsightID,
		ToInsightID:   rel.ToInsightID,
		Type:          string(rel.Type),
		Confidence:    rel.Confidence,
		Rationale:     rel.Rationale,
		DiscoveredAt:  rel.DiscoveredAt,
	}

	for _, edgeSK := range [2]string{
		relSK(rel.FromInsightID, rel.ToInsightID),
		relSK(rel.ToInsightID, rel.FromInsightID),
	} {
		item.SK = edgeSK
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

func (r *InsightAdapter) insightExists(ctx context.Context, tenantID, insightID string) (bool, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk(tenantID)},
			"sk": &types.AttributeValueMemberS{Value: sk(insightID)},
		},
	})
	if err != nil {
		return false, err
	}
	return out.Item != nil, nil
}
