package dynamodb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

func TestInsightAdapter_Put_HappyPath_WritesBidirectionalEdge(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTestAdapter(f, now)

	for _, insight := range []domain.Insight{
		{ID: "i-1", TenantID: "t-1", Text: "hello from one"},
		{ID: "i-2", TenantID: "t-1", Text: "hello from two"},
	} {
		if _, err := a.CreateIfAbsent(ctx, insight); err != nil {
			t.Fatalf("CreateIfAbsent(%s): %v", insight.ID, err)
		}
	}

	rel := domain.Relationship{
		TenantID:      "t-1",
		FromInsightID: "i-1",
		ToInsightID:   "i-2",
		Type:          domain.RelationSupports,
		Confidence:    0.9,
		Rationale:     "because reasons",
		DiscoveredAt:  now,
	}
	if err := a.Put(ctx, rel); err != nil {
		t.Fatalf("Put: %v", err)
	}

	forward, ok := f.items[pk("t-1")+"|"+relSK("i-1", "i-2")]
	if !ok {
		t.Fatalf("forward edge item missing")
	}
	if strAttr(forward, "rationale") != "because reasons" {
		t.Fatalf("forward rationale = %q, want unmodified", strAttr(forward, "rationale"))
	}
	// The forward item is queried from i-1's side, so the denormalized text
	// must be the *other* insight's (i-2's).
	if strAttr(forward, "related_insight_text") != "hello from two" {
		t.Fatalf("forward related_insight_text = %q, want i-2's text", strAttr(forward, "related_insight_text"))
	}

	reverse, ok := f.items[pk("t-1")+"|"+relSK("i-2", "i-1")]
	if !ok {
		t.Fatalf("reverse edge item missing")
	}
	// Both copies keep the edge's original direction so a reader always
	// knows which insight was "from" regardless of which side it queried.
	if strAttr(reverse, "from_insight_id") != "i-1" || strAttr(reverse, "to_insight_id") != "i-2" {
		t.Fatalf("reverse item direction = from=%q to=%q, want from=i-1 to=i-2", strAttr(reverse, "from_insight_id"), strAttr(reverse, "to_insight_id"))
	}
	// The reverse item is queried from i-2's side, so its denormalized text
	// must be i-1's.
	if strAttr(reverse, "related_insight_text") != "hello from one" {
		t.Fatalf("reverse related_insight_text = %q, want i-1's text", strAttr(reverse, "related_insight_text"))
	}
}

func TestInsightAdapter_Put_UnknownInsight_ReturnsErrInsightNotFound(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	a := newTestAdapter(f, time.Now())

	if _, err := a.CreateIfAbsent(ctx, domain.Insight{ID: "i-1", TenantID: "t-1", Text: "hello"}); err != nil {
		t.Fatalf("CreateIfAbsent: %v", err)
	}

	rel := domain.Relationship{
		TenantID:      "t-1",
		FromInsightID: "i-1",
		ToInsightID:   "i-missing",
		Type:          domain.RelationSupports,
		Confidence:    0.9,
	}
	err := a.Put(ctx, rel)
	if !errors.Is(err, ports.ErrInsightNotFound) {
		t.Fatalf("Put err = %v, want ErrInsightNotFound", err)
	}
}

func TestInsightAdapter_Put_Duplicate_UpdatesRatherThanDuplicating(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	a := newTestAdapter(f, time.Now())

	for _, id := range []string{"i-1", "i-2"} {
		if _, err := a.CreateIfAbsent(ctx, domain.Insight{ID: id, TenantID: "t-1", Text: "hello"}); err != nil {
			t.Fatalf("CreateIfAbsent(%s): %v", id, err)
		}
	}

	first := domain.Relationship{TenantID: "t-1", FromInsightID: "i-1", ToInsightID: "i-2", Type: domain.RelationSupports, Confidence: 0.7, Rationale: "v1"}
	if err := a.Put(ctx, first); err != nil {
		t.Fatalf("first Put: %v", err)
	}

	second := first
	second.Rationale = "v2"
	second.Confidence = 0.95
	if err := a.Put(ctx, second); err != nil {
		t.Fatalf("second Put: %v", err)
	}

	if len(f.items) != 4 { // 2 insight items + 2 edge items (forward+reverse), never 4 edge items
		t.Fatalf("len(items) = %d, want 4 (no duplicate edge items)", len(f.items))
	}
	forward := f.items[pk("t-1")+"|"+relSK("i-1", "i-2")]
	if strAttr(forward, "rationale") != "v2" {
		t.Fatalf("forward rationale = %q, want updated to v2", strAttr(forward, "rationale"))
	}
}

func TestInsightAdapter_ListByInsightID_MergesBothDirections_SortedByConfidenceDesc_NoExtraFetch(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	a := newTestAdapter(f, time.Now())

	for _, insight := range []domain.Insight{
		{ID: "i-1", TenantID: "t-1", Text: "one"},
		{ID: "i-2", TenantID: "t-1", Text: "two"},
		{ID: "i-3", TenantID: "t-1", Text: "three"},
	} {
		if _, err := a.CreateIfAbsent(ctx, insight); err != nil {
			t.Fatalf("CreateIfAbsent(%s): %v", insight.ID, err)
		}
	}

	// i-1 -> i-2 (i-1 is "from"): found via i-1's forward item.
	if err := a.Put(ctx, domain.Relationship{TenantID: "t-1", FromInsightID: "i-1", ToInsightID: "i-2", Type: domain.RelationSupports, Confidence: 0.6, Rationale: "low"}); err != nil {
		t.Fatalf("Put(i-1,i-2): %v", err)
	}
	// i-3 -> i-1 (i-1 is "to"): found via i-1's reverse item, not its forward one.
	if err := a.Put(ctx, domain.Relationship{TenantID: "t-1", FromInsightID: "i-3", ToInsightID: "i-1", Type: domain.RelationContradicts, Confidence: 0.9, Rationale: "high"}); err != nil {
		t.Fatalf("Put(i-3,i-1): %v", err)
	}

	related, err := a.ListByInsightID(ctx, "t-1", "i-1")
	if err != nil {
		t.Fatalf("ListByInsightID: %v", err)
	}
	if len(related) != 2 {
		t.Fatalf("ListByInsightID = %+v, want 2 related insights", related)
	}
	if related[0].InsightID != "i-3" || related[0].Confidence != 0.9 || related[0].Text != "three" {
		t.Fatalf("related[0] = %+v, want i-3 confidence=0.9 text=three (sorted first)", related[0])
	}
	if related[1].InsightID != "i-2" || related[1].Confidence != 0.6 || related[1].Text != "two" {
		t.Fatalf("related[1] = %+v, want i-2 confidence=0.6 text=two", related[1])
	}
}

func TestInsightAdapter_ListByInsightID_NoRelationships_ReturnsEmptyNotNil(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	a := newTestAdapter(f, time.Now())

	if _, err := a.CreateIfAbsent(ctx, domain.Insight{ID: "i-1", TenantID: "t-1", Text: "hello"}); err != nil {
		t.Fatalf("CreateIfAbsent: %v", err)
	}

	related, err := a.ListByInsightID(ctx, "t-1", "i-1")
	if err != nil {
		t.Fatalf("ListByInsightID: %v", err)
	}
	if len(related) != 0 {
		t.Fatalf("ListByInsightID = %v, want empty", related)
	}
}
