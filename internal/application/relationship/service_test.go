package relationship

import (
	"context"
	"errors"
	"testing"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

type spyRepo struct {
	putErr        error
	gotPutRel     domain.Relationship
	putCalled     bool
	listRelated   []domain.RelatedInsight
	listTenantID  string
	listInsightID string
}

func (s *spyRepo) Put(_ context.Context, rel domain.Relationship) error {
	s.putCalled = true
	s.gotPutRel = rel
	return s.putErr
}

func (s *spyRepo) ListByInsightID(_ context.Context, tenantID, insightID string) ([]domain.RelatedInsight, error) {
	s.listTenantID = tenantID
	s.listInsightID = insightID
	return s.listRelated, nil
}

type spyEventPublisher struct {
	err       error
	published []domain.DomainEvent
}

func (s *spyEventPublisher) Publish(_ context.Context, event domain.DomainEvent) error {
	s.published = append(s.published, event)
	return s.err
}

func makeRelationship() domain.Relationship {
	return domain.Relationship{
		TenantID:      "t-1",
		FromInsightID: "i-1",
		ToInsightID:   "i-2",
		Type:          domain.RelationSupports,
		Confidence:    0.9,
		Rationale:     "because reasons",
	}
}

func TestService_Put_HappyPath_PersistsThenPublishesKnowledgeUpdated(t *testing.T) {
	repo := &spyRepo{}
	pub := &spyEventPublisher{}
	svc := NewService(repo, pub)

	rel := makeRelationship()
	if err := svc.Put(context.Background(), rel); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if !repo.putCalled || repo.gotPutRel != rel {
		t.Fatalf("expected repo.Put(%+v), got called=%v with %+v", rel, repo.putCalled, repo.gotPutRel)
	}
	if len(pub.published) != 1 || pub.published[0].EventType != domain.KnowledgeUpdated {
		t.Fatalf("published = %+v, want single KnowledgeUpdated event", pub.published)
	}
	if pub.published[0].TenantID != "t-1" {
		t.Fatalf("event TenantID = %q, want t-1", pub.published[0].TenantID)
	}
	payload, ok := pub.published[0].Payload.(domain.KnowledgeUpdatedPayload)
	if !ok || payload.FromInsightID != "i-1" || payload.ToInsightID != "i-2" {
		t.Fatalf("event payload = %+v, want from=i-1 to=i-2", pub.published[0].Payload)
	}
}

func TestService_Put_RepoError_NeverPublishes(t *testing.T) {
	wantErr := errors.New("write failed")
	repo := &spyRepo{putErr: wantErr}
	pub := &spyEventPublisher{}
	svc := NewService(repo, pub)

	err := svc.Put(context.Background(), makeRelationship())
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if len(pub.published) != 0 {
		t.Fatalf("expected no events published, got %+v", pub.published)
	}
}

func TestService_Put_PublishError_Propagates(t *testing.T) {
	wantErr := errors.New("publish failed")
	repo := &spyRepo{}
	pub := &spyEventPublisher{err: wantErr}
	svc := NewService(repo, pub)

	err := svc.Put(context.Background(), makeRelationship())
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if !repo.putCalled {
		t.Fatalf("expected repo.Put to still have been called before the publish failure")
	}
}

func TestService_ListByInsightID_DelegatesToRepo(t *testing.T) {
	want := []domain.RelatedInsight{{InsightID: "i-2", Text: "hello"}}
	repo := &spyRepo{listRelated: want}
	svc := NewService(repo, &spyEventPublisher{})

	got, err := svc.ListByInsightID(context.Background(), "t-1", "i-1")
	if err != nil {
		t.Fatalf("ListByInsightID: %v", err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("got = %+v, want %+v", got, want)
	}
	if repo.listTenantID != "t-1" || repo.listInsightID != "i-1" {
		t.Fatalf("repo received tenant=%q insight=%q, want t-1/i-1", repo.listTenantID, repo.listInsightID)
	}
}
