package weeklyplan

import (
	"context"
	"errors"
	"testing"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

type spyRepo struct {
	createErr error
	gotCreate domain.WeeklyPlan

	getPlan domain.WeeklyPlan
	getErr  error

	listPlans []domain.WeeklyPlan
	listErr   error

	setReadyErr     error
	setReadyActions []domain.Action

	setFailedErr    error
	setFailedReason string
}

func (s *spyRepo) Create(_ context.Context, plan domain.WeeklyPlan) error {
	s.gotCreate = plan
	return s.createErr
}

func (s *spyRepo) Get(_ context.Context, _, _ string) (domain.WeeklyPlan, error) {
	return s.getPlan, s.getErr
}

func (s *spyRepo) ListPlansByTenantID(_ context.Context, _ string) ([]domain.WeeklyPlan, error) {
	return s.listPlans, s.listErr
}

func (s *spyRepo) SetReady(_ context.Context, _, _ string, actions []domain.Action) error {
	s.setReadyActions = actions
	return s.setReadyErr
}

func (s *spyRepo) SetFailed(_ context.Context, _, _, reason string) error {
	s.setFailedReason = reason
	return s.setFailedErr
}

type fakeInsightRepo struct {
	byTagAndTenant map[string][]domain.Insight // key: tenantID + "|" + tag
}

func (f *fakeInsightRepo) CreateIfAbsent(context.Context, domain.Insight) (bool, error) {
	return false, nil
}
func (f *fakeInsightRepo) Update(context.Context, domain.Insight) error { return nil }
func (f *fakeInsightRepo) ListByTenantID(_ context.Context, tenantID, tag string) ([]domain.Insight, error) {
	return f.byTagAndTenant[tenantID+"|"+tag], nil
}
func (f *fakeInsightRepo) ListByTag(context.Context, string, string) ([]domain.TagMembership, error) {
	return nil, nil
}
func (f *fakeInsightRepo) ListTags(context.Context, string) ([]domain.TagSummary, error) {
	return nil, nil
}

type spyEventPublisher struct {
	err       error
	published []domain.DomainEvent
}

func (s *spyEventPublisher) Publish(_ context.Context, event domain.DomainEvent) error {
	s.published = append(s.published, event)
	return s.err
}

func TestService_Submit_HappyPath_PersistsThenPublishesWeeklyPlanRequested(t *testing.T) {
	repo := &spyRepo{}
	pub := &spyEventPublisher{}
	svc := NewService(repo, &fakeInsightRepo{}, pub)

	plan := domain.WeeklyPlan{ID: "p-1", TenantID: "t-1", Tag: "golang", FocusSentence: "focus", Status: domain.PlanStatusPending}
	if err := svc.Submit(context.Background(), plan); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if repo.gotCreate.ID != plan.ID || repo.gotCreate.TenantID != plan.TenantID || repo.gotCreate.Tag != plan.Tag {
		t.Fatalf("repo.Create got %+v, want %+v", repo.gotCreate, plan)
	}
	if len(pub.published) != 1 || pub.published[0].EventType != domain.WeeklyPlanRequested {
		t.Fatalf("published = %+v, want single WeeklyPlanRequested event", pub.published)
	}
}

func TestService_Submit_RepoError_NeverPublishes(t *testing.T) {
	wantErr := errors.New("write failed")
	repo := &spyRepo{createErr: wantErr}
	pub := &spyEventPublisher{}
	svc := NewService(repo, &fakeInsightRepo{}, pub)

	err := svc.Submit(context.Background(), domain.WeeklyPlan{ID: "p-1", TenantID: "t-1"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if len(pub.published) != 0 {
		t.Fatalf("expected no events published, got %+v", pub.published)
	}
}

func TestService_Get_ResolvesSupportingInsightIDsAgainstThePlansTag(t *testing.T) {
	repo := &spyRepo{getPlan: domain.WeeklyPlan{
		ID: "p-1", TenantID: "t-1", Tag: "golang", Status: domain.PlanStatusReady,
		Actions: []domain.Action{
			{Title: "Ship it", Why: "why", SupportingInsightIDs: []string{"i-1", "i-2"}},
		},
	}}
	insights := &fakeInsightRepo{byTagAndTenant: map[string][]domain.Insight{
		"t-1|golang": {
			{ID: "i-1", Text: "insight one"},
			{ID: "i-3", Text: "unrelated"},
		},
	}}
	svc := NewService(repo, insights, &spyEventPublisher{})

	detail, err := svc.Get(context.Background(), "t-1", "p-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if len(detail.Actions) != 1 {
		t.Fatalf("Actions = %+v, want exactly one", detail.Actions)
	}
	supporting := detail.Actions[0].SupportingInsights
	// i-1 resolves (it's in the tag's pool); i-2 doesn't (deleted since the
	// plan was generated) and is silently dropped, same as an orphaned tag
	// membership elsewhere in the codebase.
	if len(supporting) != 1 || supporting[0].InsightID != "i-1" || supporting[0].Text != "insight one" {
		t.Fatalf("SupportingInsights = %+v, want only the resolvable i-1", supporting)
	}
}

func TestService_Get_NoActions_NeverQueriesInsights(t *testing.T) {
	repo := &spyRepo{getPlan: domain.WeeklyPlan{ID: "p-1", TenantID: "t-1", Tag: "golang", Status: domain.PlanStatusPending}}
	svc := NewService(repo, &fakeInsightRepo{}, &spyEventPublisher{}) // empty map: ListByTenantID would return nil either way, but the point is it must not be reached with a panic-worthy nil map lookup

	detail, err := svc.Get(context.Background(), "t-1", "p-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(detail.Actions) != 0 {
		t.Fatalf("Actions = %+v, want none for a pending plan", detail.Actions)
	}
}

func TestService_Get_RepoError_Propagates(t *testing.T) {
	wantErr := errors.New("not found")
	repo := &spyRepo{getErr: wantErr}
	svc := NewService(repo, &fakeInsightRepo{}, &spyEventPublisher{})

	_, err := svc.Get(context.Background(), "t-1", "p-missing")
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestService_List_DelegatesToRepo(t *testing.T) {
	want := []domain.WeeklyPlan{{ID: "p-1"}}
	repo := &spyRepo{listPlans: want}
	svc := NewService(repo, &fakeInsightRepo{}, &spyEventPublisher{})

	got, err := svc.List(context.Background(), "t-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].ID != "p-1" {
		t.Fatalf("got = %+v, want %+v", got, want)
	}
}

func TestService_Status_ReturnsThePlansStatusWithoutResolvingActions(t *testing.T) {
	repo := &spyRepo{getPlan: domain.WeeklyPlan{ID: "p-1", TenantID: "t-1", Status: domain.PlanStatusReady}}
	svc := NewService(repo, &fakeInsightRepo{}, &spyEventPublisher{})

	status, err := svc.Status(context.Background(), "t-1", "p-1")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status != domain.PlanStatusReady {
		t.Fatalf("status = %q, want %q", status, domain.PlanStatusReady)
	}
}

func TestService_Status_RepoError_Propagates(t *testing.T) {
	wantErr := errors.New("not found")
	repo := &spyRepo{getErr: wantErr}
	svc := NewService(repo, &fakeInsightRepo{}, &spyEventPublisher{})

	_, err := svc.Status(context.Background(), "t-1", "p-missing")
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestService_SetReady_DelegatesToRepo(t *testing.T) {
	repo := &spyRepo{}
	svc := NewService(repo, &fakeInsightRepo{}, &spyEventPublisher{})

	actions := []domain.Action{{Title: "Ship it"}}
	if err := svc.SetReady(context.Background(), "t-1", "p-1", actions); err != nil {
		t.Fatalf("SetReady: %v", err)
	}
	if len(repo.setReadyActions) != 1 || repo.setReadyActions[0].Title != "Ship it" {
		t.Fatalf("repo.setReadyActions = %+v", repo.setReadyActions)
	}
}

func TestService_SetFailed_DelegatesToRepo(t *testing.T) {
	repo := &spyRepo{}
	svc := NewService(repo, &fakeInsightRepo{}, &spyEventPublisher{})

	if err := svc.SetFailed(context.Background(), "t-1", "p-1", "timed out"); err != nil {
		t.Fatalf("SetFailed: %v", err)
	}
	if repo.setFailedReason != "timed out" {
		t.Fatalf("repo.setFailedReason = %q, want %q", repo.setFailedReason, "timed out")
	}
}
