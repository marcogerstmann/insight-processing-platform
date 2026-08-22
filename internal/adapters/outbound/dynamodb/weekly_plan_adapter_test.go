package dynamodb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

func TestInsightAdapter_Create_HappyPath_WritesPlanItem(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTestAdapter(f, now)

	insight := domain.Insight{ID: "i-1", TenantID: "t-1", Text: "hello"}
	if _, err := a.CreateIfAbsent(ctx, insight); err != nil {
		t.Fatalf("CreateIfAbsent: %v", err)
	}
	insight.Enrichment = &domain.Enrichment{Tags: []string{"golang"}}
	if err := a.Update(ctx, insight); err != nil {
		t.Fatalf("Update: %v", err)
	}

	plan := domain.WeeklyPlan{
		ID:            "p-1",
		TenantID:      "t-1",
		Tag:           "golang",
		FocusSentence: "Read more about distributed systems this week.",
		Status:        domain.PlanStatusPending,
		CreatedAt:     now,
	}
	if err := a.Create(ctx, plan); err != nil {
		t.Fatalf("Create: %v", err)
	}

	item, ok := f.items[pk("t-1")+"|"+planSK("p-1")]
	if !ok {
		t.Fatalf("plan item missing")
	}
	if strAttr(item, "status") != "pending" || strAttr(item, "tag") != "golang" {
		t.Fatalf("item = %+v, want status=pending tag=golang", item)
	}
}

func TestInsightAdapter_Create_UnknownTag_ReturnsErrUnknownTag(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	a := newTestAdapter(f, time.Now())

	plan := domain.WeeklyPlan{
		ID:            "p-1",
		TenantID:      "t-1",
		Tag:           "nonexistent",
		FocusSentence: "Read more about distributed systems this week.",
		Status:        domain.PlanStatusPending,
	}
	err := a.Create(ctx, plan)
	if !errors.Is(err, ports.ErrUnknownTag) {
		t.Fatalf("Create err = %v, want ErrUnknownTag", err)
	}
	if _, ok := f.items[pk("t-1")+"|"+planSK("p-1")]; ok {
		t.Fatalf("plan item should not have been written for an unknown tag")
	}
}
