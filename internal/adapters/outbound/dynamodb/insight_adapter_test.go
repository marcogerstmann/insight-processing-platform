package dynamodb

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

// fakeDynamo is a minimal in-memory stand-in for *dynamodb.Client, covering
// only the request shapes InsightAdapter actually sends. No dynamodb-local
// in this repo, so this plays the same role the httptest fakes play for the
// readwise HTTP client.
type fakeDynamo struct {
	items map[string]map[string]types.AttributeValue // key: pk|sk
	index map[string]map[string]types.AttributeValue // key: gsi1pk|gsi1sk
}

func newFakeDynamo() *fakeDynamo {
	return &fakeDynamo{
		items: map[string]map[string]types.AttributeValue{},
		index: map[string]map[string]types.AttributeValue{},
	}
}

func strAttr(item map[string]types.AttributeValue, name string) string {
	v, ok := item[name].(*types.AttributeValueMemberS)
	if !ok {
		return ""
	}
	return v.Value
}

func compositeKey(item map[string]types.AttributeValue, pkAttr, skAttr string) string {
	return strAttr(item, pkAttr) + "|" + strAttr(item, skAttr)
}

func (f *fakeDynamo) PutItem(_ context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	key := compositeKey(in.Item, "pk", "sk")
	if in.ConditionExpression != nil && strings.Contains(*in.ConditionExpression, "attribute_not_exists") {
		if _, exists := f.items[key]; exists {
			return nil, &types.ConditionalCheckFailedException{}
		}
	}
	f.items[key] = in.Item
	if _, ok := in.Item["gsi1pk"]; ok {
		f.index[compositeKey(in.Item, "gsi1pk", "gsi1sk")] = in.Item
	}
	return &dynamodb.PutItemOutput{}, nil
}

func (f *fakeDynamo) GetItem(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	item, ok := f.items[compositeKey(in.Key, "pk", "sk")]
	if !ok {
		return &dynamodb.GetItemOutput{}, nil
	}
	return &dynamodb.GetItemOutput{Item: item}, nil
}

func (f *fakeDynamo) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	key := compositeKey(in.Key, "pk", "sk")
	item, exists := f.items[key]
	if !exists {
		return nil, &types.ConditionalCheckFailedException{}
	}

	expr := strings.TrimPrefix(*in.UpdateExpression, "SET ")
	for _, clause := range strings.Split(expr, ", ") {
		parts := strings.SplitN(clause, " = ", 2)
		attrName := in.ExpressionAttributeNames[parts[0]]
		item[attrName] = in.ExpressionAttributeValues[parts[1]]
	}

	if _, ok := item["gsi1pk"]; ok {
		f.index[compositeKey(item, "gsi1pk", "gsi1sk")] = item
	}
	return &dynamodb.UpdateItemOutput{}, nil
}

func (f *fakeDynamo) DeleteItem(_ context.Context, in *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	key := compositeKey(in.Key, "pk", "sk")
	if item, ok := f.items[key]; ok {
		if _, ok := item["gsi1pk"]; ok {
			delete(f.index, compositeKey(item, "gsi1pk", "gsi1sk"))
		}
	}
	delete(f.items, key)
	return &dynamodb.DeleteItemOutput{}, nil
}

func (f *fakeDynamo) Query(_ context.Context, in *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	source := f.items
	pkAttr, skAttr := "pk", "sk"
	if in.IndexName != nil {
		source = f.index
		pkAttr, skAttr = "gsi1pk", "gsi1sk"
	}

	pkVal := in.ExpressionAttributeValues[":pk"].(*types.AttributeValueMemberS).Value
	var skPrefix string
	if v, ok := in.ExpressionAttributeValues[":skPrefix"]; ok {
		skPrefix = v.(*types.AttributeValueMemberS).Value
	}

	var matched []map[string]types.AttributeValue
	for _, item := range source {
		if strAttr(item, pkAttr) != pkVal {
			continue
		}
		if skPrefix != "" && !strings.HasPrefix(strAttr(item, skAttr), skPrefix) {
			continue
		}
		matched = append(matched, item)
	}
	sort.Slice(matched, func(i, j int) bool {
		return strAttr(matched[i], skAttr) < strAttr(matched[j], skAttr)
	})

	return &dynamodb.QueryOutput{Items: matched, Count: int32(len(matched))}, nil
}

func newTestAdapter(f *fakeDynamo, fixedNow time.Time) *InsightAdapter {
	a := NewInsightAdapter(f, "test-table")
	a.now = func() time.Time { return fixedNow }
	return a
}

func TestInsightAdapter_Update_WritesTagMembershipItems(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTestAdapter(f, now)

	insight := domain.Insight{ID: "i-1", TenantID: "t-1", Source: "readwise", Text: "hello"}
	if _, err := a.CreateIfAbsent(ctx, insight); err != nil {
		t.Fatalf("CreateIfAbsent: %v", err)
	}

	insight.Enrichment = &domain.Enrichment{Tags: []string{"a", "b"}}
	if err := a.Update(ctx, insight); err != nil {
		t.Fatalf("Update: %v", err)
	}

	for _, tag := range []string{"a", "b"} {
		members, err := a.ListByTag(ctx, "t-1", tag)
		if err != nil {
			t.Fatalf("ListByTag(%q): %v", tag, err)
		}
		if len(members) != 1 || members[0].InsightID != "i-1" {
			t.Fatalf("ListByTag(%q) = %v, want single membership for i-1", tag, members)
		}
		if !members[0].CreatedAt.Equal(now) {
			t.Fatalf("ListByTag(%q) CreatedAt = %v, want %v", tag, members[0].CreatedAt, now)
		}
	}

	// Sparse GSI: the plain insight listing must not be polluted by tag
	// membership items sharing the same tenant partition.
	insights, err := a.ListByTenantID(ctx, "t-1", "")
	if err != nil {
		t.Fatalf("ListByTenantID: %v", err)
	}
	if len(insights) != 1 || insights[0].ID != "i-1" {
		t.Fatalf("ListByTenantID = %v, want single insight i-1", insights)
	}
}

func TestInsightAdapter_Update_RewriteWithChangedTags_RemovesOrphansKeepsUnchanged(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTestAdapter(f, t1)

	insight := domain.Insight{ID: "i-1", TenantID: "t-1", Source: "readwise", Text: "hello"}
	if _, err := a.CreateIfAbsent(ctx, insight); err != nil {
		t.Fatalf("CreateIfAbsent: %v", err)
	}

	insight.Enrichment = &domain.Enrichment{Tags: []string{"a", "b"}}
	if err := a.Update(ctx, insight); err != nil {
		t.Fatalf("first Update: %v", err)
	}

	t2 := t1.Add(time.Hour)
	a.now = func() time.Time { return t2 }
	insight.Enrichment = &domain.Enrichment{Tags: []string{"b", "c"}}
	if err := a.Update(ctx, insight); err != nil {
		t.Fatalf("second Update: %v", err)
	}

	if members, err := a.ListByTag(ctx, "t-1", "a"); err != nil || len(members) != 0 {
		t.Fatalf("ListByTag(a) = %v, err=%v, want no orphaned membership", members, err)
	}

	cMembers, err := a.ListByTag(ctx, "t-1", "c")
	if err != nil {
		t.Fatalf("ListByTag(c): %v", err)
	}
	if len(cMembers) != 1 || !cMembers[0].CreatedAt.Equal(t2) {
		t.Fatalf("ListByTag(c) = %v, want single membership created at %v", cMembers, t2)
	}

	bMembers, err := a.ListByTag(ctx, "t-1", "b")
	if err != nil {
		t.Fatalf("ListByTag(b): %v", err)
	}
	if len(bMembers) != 1 || !bMembers[0].CreatedAt.Equal(t1) {
		t.Fatalf("ListByTag(b) = %v, want single membership untouched at %v (idempotent re-write)", bMembers, t1)
	}
}

func TestInsightAdapter_ListByTag_ScopedByTenant(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTestAdapter(f, now)

	for _, tenantID := range []string{"t-1", "t-2"} {
		insight := domain.Insight{ID: "i-" + tenantID, TenantID: tenantID, Source: "readwise", Text: "hello"}
		if _, err := a.CreateIfAbsent(ctx, insight); err != nil {
			t.Fatalf("CreateIfAbsent(%s): %v", tenantID, err)
		}
		insight.Enrichment = &domain.Enrichment{Tags: []string{"shared"}}
		if err := a.Update(ctx, insight); err != nil {
			t.Fatalf("Update(%s): %v", tenantID, err)
		}
	}

	members, err := a.ListByTag(ctx, "t-1", "shared")
	if err != nil {
		t.Fatalf("ListByTag: %v", err)
	}
	if len(members) != 1 || members[0].InsightID != "i-t-1" {
		t.Fatalf("ListByTag(t-1, shared) = %v, want only tenant t-1's insight", members)
	}
}

func TestInsightAdapter_ListTags_AggregatesCountsSortsByScoreDesc_ScopedByTenant(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTestAdapter(f, t1)

	// t-1: "a" tagged twice (second write is newer), "b" tagged once.
	i1 := domain.Insight{ID: "i-1", TenantID: "t-1", Source: "readwise", Text: "hello"}
	if _, err := a.CreateIfAbsent(ctx, i1); err != nil {
		t.Fatalf("CreateIfAbsent(i-1): %v", err)
	}
	i1.Enrichment = &domain.Enrichment{Tags: []string{"a", "b"}}
	if err := a.Update(ctx, i1); err != nil {
		t.Fatalf("Update(i-1): %v", err)
	}

	t2 := t1.Add(time.Hour)
	a.now = func() time.Time { return t2 }
	i2 := domain.Insight{ID: "i-2", TenantID: "t-1", Source: "readwise", Text: "world"}
	if _, err := a.CreateIfAbsent(ctx, i2); err != nil {
		t.Fatalf("CreateIfAbsent(i-2): %v", err)
	}
	i2.Enrichment = &domain.Enrichment{Tags: []string{"a"}}
	if err := a.Update(ctx, i2); err != nil {
		t.Fatalf("Update(i-2): %v", err)
	}

	// t-2: unrelated tenant, must not leak into t-1's tags.
	other := domain.Insight{ID: "i-other", TenantID: "t-2", Source: "readwise", Text: "hi"}
	if _, err := a.CreateIfAbsent(ctx, other); err != nil {
		t.Fatalf("CreateIfAbsent(i-other): %v", err)
	}
	other.Enrichment = &domain.Enrichment{Tags: []string{"c"}}
	if err := a.Update(ctx, other); err != nil {
		t.Fatalf("Update(i-other): %v", err)
	}

	tags, err := a.ListTags(ctx, "t-1")
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("ListTags = %v, want 2 tags", tags)
	}
	if tags[0].Tag != "a" || tags[0].InsightCount != 2 || !tags[0].LastInsightAt.Equal(t2) {
		t.Fatalf("tags[0] = %+v, want tag=a count=2 lastInsightAt=%v", tags[0], t2)
	}
	if tags[1].Tag != "b" || tags[1].InsightCount != 1 || !tags[1].LastInsightAt.Equal(t1) {
		t.Fatalf("tags[1] = %+v, want tag=b count=1 lastInsightAt=%v", tags[1], t1)
	}
	if tags[0].Score <= tags[1].Score {
		t.Fatalf("tags[0].Score = %v, want > tags[1].Score = %v", tags[0].Score, tags[1].Score)
	}
}

func TestInsightAdapter_ListTags_UsesHighlightedAtNotIngestionTime(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	ingestedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTestAdapter(f, ingestedAt)

	// Readwise says this highlight is a year old, even though we're
	// ingesting it (and thus the wall-clock "now") today. Relevance
	// scoring must rank by the old highlight date, not ingestion time.
	highlightedAt := ingestedAt.Add(-365 * 24 * time.Hour)
	insight := domain.Insight{ID: "i-1", TenantID: "t-1", Source: "readwise", Text: "hello", HighlightedAt: highlightedAt}
	if _, err := a.CreateIfAbsent(ctx, insight); err != nil {
		t.Fatalf("CreateIfAbsent: %v", err)
	}
	insight.Enrichment = &domain.Enrichment{Tags: []string{"old"}}
	if err := a.Update(ctx, insight); err != nil {
		t.Fatalf("Update: %v", err)
	}

	members, err := a.ListByTag(ctx, "t-1", "old")
	if err != nil {
		t.Fatalf("ListByTag: %v", err)
	}
	if len(members) != 1 || !members[0].HighlightedAt.Equal(highlightedAt) {
		t.Fatalf("ListByTag = %+v, want HighlightedAt=%v (Readwise's highlighted_at, not ingestion now=%v)", members, highlightedAt, ingestedAt)
	}
	if !members[0].CreatedAt.Equal(ingestedAt) {
		t.Fatalf("ListByTag = %+v, want CreatedAt=%v (our own audit trail, untouched by the source timestamp)", members, ingestedAt)
	}
}

func TestInsightAdapter_ListTags_FallsBackToIngestionTimeWhenHighlightedAtUnset(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	ingestedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTestAdapter(f, ingestedAt)

	// Manually-created insights have no source highlight date.
	insight := domain.Insight{ID: "i-1", TenantID: "t-1", Source: "manual", Text: "hello"}
	if _, err := a.CreateIfAbsent(ctx, insight); err != nil {
		t.Fatalf("CreateIfAbsent: %v", err)
	}
	insight.Enrichment = &domain.Enrichment{Tags: []string{"manual"}}
	if err := a.Update(ctx, insight); err != nil {
		t.Fatalf("Update: %v", err)
	}

	members, err := a.ListByTag(ctx, "t-1", "manual")
	if err != nil {
		t.Fatalf("ListByTag: %v", err)
	}
	if len(members) != 1 || !members[0].HighlightedAt.Equal(ingestedAt) {
		t.Fatalf("ListByTag = %+v, want HighlightedAt=%v (fallback to ingestion time)", members, ingestedAt)
	}
}

func TestInsightAdapter_ListByTenantID_WithTag_ReturnsFullMatchingInsights(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTestAdapter(f, now)

	i1 := domain.Insight{ID: "i-1", TenantID: "t-1", Source: "readwise", Text: "hello"}
	if _, err := a.CreateIfAbsent(ctx, i1); err != nil {
		t.Fatalf("CreateIfAbsent(i-1): %v", err)
	}
	i1.Enrichment = &domain.Enrichment{Tags: []string{"a"}}
	if err := a.Update(ctx, i1); err != nil {
		t.Fatalf("Update(i-1): %v", err)
	}

	i2 := domain.Insight{ID: "i-2", TenantID: "t-1", Source: "readwise", Text: "world"}
	if _, err := a.CreateIfAbsent(ctx, i2); err != nil {
		t.Fatalf("CreateIfAbsent(i-2): %v", err)
	}
	i2.Enrichment = &domain.Enrichment{Tags: []string{"b"}}
	if err := a.Update(ctx, i2); err != nil {
		t.Fatalf("Update(i-2): %v", err)
	}

	insights, err := a.ListByTenantID(ctx, "t-1", "a")
	if err != nil {
		t.Fatalf("ListByTenantID(tag=a): %v", err)
	}
	if len(insights) != 1 || insights[0].ID != "i-1" || insights[0].Text != "hello" {
		t.Fatalf("ListByTenantID(tag=a) = %v, want full insight i-1", insights)
	}
	if insights[0].Enrichment == nil || len(insights[0].Enrichment.Tags) != 1 || insights[0].Enrichment.Tags[0] != "a" {
		t.Fatalf("ListByTenantID(tag=a) enrichment = %+v, want tags=[a]", insights[0].Enrichment)
	}
}

func TestInsightAdapter_ListByTenantID_UnknownTag_ReturnsEmptyNotNil(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	a := newTestAdapter(f, time.Now())

	insight := domain.Insight{ID: "i-1", TenantID: "t-1", Source: "readwise", Text: "hello"}
	if _, err := a.CreateIfAbsent(ctx, insight); err != nil {
		t.Fatalf("CreateIfAbsent: %v", err)
	}
	insight.Enrichment = &domain.Enrichment{Tags: []string{"a"}}
	if err := a.Update(ctx, insight); err != nil {
		t.Fatalf("Update: %v", err)
	}

	insights, err := a.ListByTenantID(ctx, "t-1", "unknown")
	if err != nil {
		t.Fatalf("ListByTenantID(tag=unknown): %v", err)
	}
	if len(insights) != 0 {
		t.Fatalf("ListByTenantID(tag=unknown) = %v, want empty", insights)
	}
}

func TestInsightAdapter_ListTags_EmptyTenant_ReturnsEmptyNotNil(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	a := newTestAdapter(f, time.Now())

	tags, err := a.ListTags(ctx, "t-empty")
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("ListTags(empty tenant) = %v, want empty", tags)
	}
}
