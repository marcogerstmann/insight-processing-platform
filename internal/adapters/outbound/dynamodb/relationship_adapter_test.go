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

	for _, id := range []string{"i-1", "i-2"} {
		if _, err := a.CreateIfAbsent(ctx, domain.Insight{ID: id, TenantID: "t-1", Text: "hello"}); err != nil {
			t.Fatalf("CreateIfAbsent(%s): %v", id, err)
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

	reverse, ok := f.items[pk("t-1")+"|"+relSK("i-2", "i-1")]
	if !ok {
		t.Fatalf("reverse edge item missing")
	}
	// Both copies keep the edge's original direction so a reader always
	// knows which insight was "from" regardless of which side it queried.
	if strAttr(reverse, "from_insight_id") != "i-1" || strAttr(reverse, "to_insight_id") != "i-2" {
		t.Fatalf("reverse item direction = from=%q to=%q, want from=i-1 to=i-2", strAttr(reverse, "from_insight_id"), strAttr(reverse, "to_insight_id"))
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
