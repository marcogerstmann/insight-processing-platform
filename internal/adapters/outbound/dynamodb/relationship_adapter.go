package dynamodb

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
// FromInsightID/ToInsightID; only the sort key and RelatedInsightText
// differ (see Put).
//
// TRADE-OFF (IPP-102): RelatedInsightText duplicates the *other* insight's
// text onto the edge at write time, so GET .../relationships (REL 6) never
// needs an N+1 fetch to render a summary per edge. This goes stale if the
// insight's text ever changes after the edge is written — acceptable here
// since insight text is immutable post-ingestion in this codebase.
type dynamoRelationshipItem struct {
	PK                 string    `dynamodbav:"pk"`
	SK                 string    `dynamodbav:"sk"`
	TenantID           string    `dynamodbav:"tenant_id"`
	FromInsightID      string    `dynamodbav:"from_insight_id"`
	ToInsightID        string    `dynamodbav:"to_insight_id"`
	RelatedInsightText string    `dynamodbav:"related_insight_text"`
	Type               string    `dynamodbav:"type"`
	Confidence         float64   `dynamodbav:"confidence"`
	Rationale          string    `dynamodbav:"rationale"`
	DiscoveredAt       time.Time `dynamodbav:"discovered_at"`
}

func relSK(fromInsightID, toInsightID string) string {
	return "REL#" + fromInsightID + "#" + toInsightID
}

// parseRelOwnerFromSK extracts the owning insight ID from a relationship
// item's sort key ("REL#<ownerID>#<otherID>"): whichever insight this
// particular copy of the edge is filed under (see relSKPrefix and Put).
// Used by relationshipDegreeByInsight to count edges per insight from a
// single tenant-wide query.
func parseRelOwnerFromSK(sk string) (string, bool) {
	rest, ok := strings.CutPrefix(sk, "REL#")
	if !ok {
		return "", false
	}
	owner, _, ok := strings.Cut(rest, "#")
	return owner, ok
}

func relSKPrefix(insightID string) string {
	return "REL#" + insightID + "#"
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
	fromInsight, err := r.getInsight(ctx, rel.TenantID, rel.FromInsightID)
	if err != nil {
		return fmt.Errorf("get from insight: %w", err)
	}
	toInsight, err := r.getInsight(ctx, rel.TenantID, rel.ToInsightID)
	if err != nil {
		return fmt.Errorf("get to insight: %w", err)
	}
	if fromInsight == nil || toInsight == nil {
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

	edges := [2]struct {
		sk          string
		relatedText string
	}{
		{relSK(rel.FromInsightID, rel.ToInsightID), toInsight.Text},
		{relSK(rel.ToInsightID, rel.FromInsightID), fromInsight.Text},
	}
	for _, edge := range edges {
		item.SK = edge.sk
		item.RelatedInsightText = edge.relatedText
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

// ListByInsightID returns insightID's edges — from either direction it was
// originally discovered in — sorted by confidence descending. A single
// begins_with(sk, "REL#<insightID>#") query: no per-edge fetch of the
// related insight, since its text was denormalized onto the edge at write
// time (see dynamoRelationshipItem's doc comment).
func (r *InsightAdapter) ListByInsightID(ctx context.Context, tenantID, insightID string) ([]domain.RelatedInsight, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("#pk = :pk AND begins_with(#sk, :skPrefix)"),
		ExpressionAttributeNames: map[string]string{
			"#pk": "pk",
			"#sk": "sk",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":       &types.AttributeValueMemberS{Value: pk(tenantID)},
			":skPrefix": &types.AttributeValueMemberS{Value: relSKPrefix(insightID)},
		},
	})
	if err != nil {
		return nil, err
	}

	related := make([]domain.RelatedInsight, 0, len(out.Items))
	for _, dynItem := range out.Items {
		var item dynamoRelationshipItem
		if err := attributevalue.UnmarshalMap(dynItem, &item); err != nil {
			return nil, err
		}

		relatedID := item.ToInsightID
		if item.FromInsightID != insightID {
			relatedID = item.FromInsightID
		}

		related = append(related, domain.RelatedInsight{
			InsightID:  relatedID,
			Text:       item.RelatedInsightText,
			Type:       domain.RelationType(item.Type),
			Confidence: item.Confidence,
			Rationale:  item.Rationale,
		})
	}

	sort.SliceStable(related, func(i, j int) bool {
		return related[i].Confidence > related[j].Confidence
	})

	return related, nil
}

func (r *InsightAdapter) getInsight(ctx context.Context, tenantID, insightID string) (*domain.Insight, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk(tenantID)},
			"sk": &types.AttributeValueMemberS{Value: sk(insightID)},
		},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, nil
	}
	insight, err := unmarshalInsight(out.Item)
	if err != nil {
		return nil, err
	}
	return &insight, nil
}
