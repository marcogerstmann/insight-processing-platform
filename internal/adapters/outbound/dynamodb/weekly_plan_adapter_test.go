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

func TestInsightAdapter_Get_UnknownPlan_ReturnsErrPlanNotFound(t *testing.T) {
	ctx := context.Background()
	a := newTestAdapter(newFakeDynamo(), time.Now())

	_, err := a.Get(ctx, "t-1", "p-missing")
	if !errors.Is(err, ports.ErrPlanNotFound) {
		t.Fatalf("Get err = %v, want ErrPlanNotFound", err)
	}
}

func TestInsightAdapter_SetReady_HappyPath_PersistsActionsAndStatus(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := newTestAdapter(f, now)

	seedTaggedInsight(t, ctx, a, "t-1", "golang")
	plan := domain.WeeklyPlan{
		ID: "p-1", TenantID: "t-1", Tag: "golang",
		FocusSentence: "focus", Status: domain.PlanStatusPending, CreatedAt: now,
	}
	if err := a.Create(ctx, plan); err != nil {
		t.Fatalf("Create: %v", err)
	}

	actions := []domain.Action{
		{Title: "Ship it", Why: "why", SupportingInsightIDs: []string{"i-1"}},
	}
	if err := a.SetReady(ctx, "t-1", "p-1", actions); err != nil {
		t.Fatalf("SetReady: %v", err)
	}

	got, err := a.Get(ctx, "t-1", "p-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != domain.PlanStatusReady {
		t.Fatalf("Status = %q, want ready", got.Status)
	}
	if len(got.Actions) != 1 || got.Actions[0].Title != "Ship it" || got.Actions[0].SupportingInsightIDs[0] != "i-1" {
		t.Fatalf("Actions = %+v, want one Ship it action citing i-1", got.Actions)
	}
}

func TestInsightAdapter_SetReady_UnknownPlan_ReturnsErrPlanNotPending(t *testing.T) {
	ctx := context.Background()
	a := newTestAdapter(newFakeDynamo(), time.Now())

	err := a.SetReady(ctx, "t-1", "p-missing", nil)
	if !errors.Is(err, ports.ErrPlanNotPending) {
		t.Fatalf("SetReady err = %v, want ErrPlanNotPending", err)
	}
}

func TestInsightAdapter_SetReady_AlreadyReady_ReturnsErrPlanNotPending(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	now := time.Now()
	a := newTestAdapter(f, now)

	seedTaggedInsight(t, ctx, a, "t-1", "golang")
	plan := domain.WeeklyPlan{ID: "p-1", TenantID: "t-1", Tag: "golang", FocusSentence: "focus", Status: domain.PlanStatusPending, CreatedAt: now}
	if err := a.Create(ctx, plan); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := a.SetReady(ctx, "t-1", "p-1", nil); err != nil {
		t.Fatalf("first SetReady: %v", err)
	}

	// The redelivery case PLAN 5 (IPP-107) leans on: a second call must not
	// silently overwrite the first result.
	err := a.SetReady(ctx, "t-1", "p-1", []domain.Action{{Title: "should not land"}})
	if !errors.Is(err, ports.ErrPlanNotPending) {
		t.Fatalf("second SetReady err = %v, want ErrPlanNotPending", err)
	}

	got, err := a.Get(ctx, "t-1", "p-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Actions) != 0 {
		t.Fatalf("Actions = %+v, want the first (empty) result to still stand", got.Actions)
	}
}

func TestInsightAdapter_SetFailed_HappyPath_PersistsReasonAndStatus(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	now := time.Now()
	a := newTestAdapter(f, now)

	seedTaggedInsight(t, ctx, a, "t-1", "golang")
	plan := domain.WeeklyPlan{ID: "p-1", TenantID: "t-1", Tag: "golang", FocusSentence: "focus", Status: domain.PlanStatusPending, CreatedAt: now}
	if err := a.Create(ctx, plan); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := a.SetFailed(ctx, "t-1", "p-1", "llm timed out"); err != nil {
		t.Fatalf("SetFailed: %v", err)
	}

	got, err := a.Get(ctx, "t-1", "p-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != domain.PlanStatusFailed || got.FailureReason != "llm timed out" {
		t.Fatalf("got = %+v, want status=failed reason=%q", got, "llm timed out")
	}
}

func TestInsightAdapter_ListPlansByTenantID_ReturnsNewestFirst(t *testing.T) {
	ctx := context.Background()
	f := newFakeDynamo()
	a := newTestAdapter(f, time.Now())

	seedTaggedInsight(t, ctx, a, "t-1", "golang")
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(24 * time.Hour)
	if err := a.Create(ctx, domain.WeeklyPlan{ID: "p-old", TenantID: "t-1", Tag: "golang", FocusSentence: "f", Status: domain.PlanStatusPending, CreatedAt: older}); err != nil {
		t.Fatalf("Create p-old: %v", err)
	}
	if err := a.Create(ctx, domain.WeeklyPlan{ID: "p-new", TenantID: "t-1", Tag: "golang", FocusSentence: "f", Status: domain.PlanStatusPending, CreatedAt: newer}); err != nil {
		t.Fatalf("Create p-new: %v", err)
	}

	plans, err := a.ListPlansByTenantID(ctx, "t-1")
	if err != nil {
		t.Fatalf("ListPlansByTenantID: %v", err)
	}
	if len(plans) != 2 || plans[0].ID != "p-new" || plans[1].ID != "p-old" {
		t.Fatalf("plans = %+v, want [p-new, p-old]", plans)
	}
}

// seedTaggedInsight makes tagExists (Create's check) pass for tag by
// writing one enriched insight carrying it — the same setup
// TestInsightAdapter_Create_HappyPath_WritesPlanItem uses.
func seedTaggedInsight(t *testing.T, ctx context.Context, a *InsightAdapter, tenantID, tag string) {
	t.Helper()
	insight := domain.Insight{ID: "i-1", TenantID: tenantID, Text: "hello"}
	if _, err := a.CreateIfAbsent(ctx, insight); err != nil {
		t.Fatalf("CreateIfAbsent: %v", err)
	}
	insight.Enrichment = &domain.Enrichment{Tags: []string{tag}}
	if err := a.Update(ctx, insight); err != nil {
		t.Fatalf("Update: %v", err)
	}
}
