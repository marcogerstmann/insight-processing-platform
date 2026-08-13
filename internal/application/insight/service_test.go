package insight

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/marcogerstmann/insight-processing-platform/internal/apperr"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/llm"
	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

type callLog struct {
	entries []string
}

func (l *callLog) add(s string) { l.entries = append(l.entries, s) }

type spyRepo struct {
	log *callLog

	putInserted bool
	putErr      error
	created     bool

	updateErr error

	gotPutInsight    domain.Insight
	gotUpdateInsight domain.Insight

	listByTenantIDInsights []domain.Insight
	gotListTag             string
	listCalled             bool
}

func (s *spyRepo) CreateIfAbsent(_ context.Context, insight domain.Insight) (bool, error) {
	if s.log != nil {
		s.log.add("repo.CreateIfAbsent")
	}
	s.gotPutInsight = insight
	if s.putErr != nil {
		return false, s.putErr
	}
	// Mimic a real repo's dedup: once inserted, later calls (e.g. an SQS
	// redelivery) report the record already exists.
	if s.created {
		return false, nil
	}
	if s.putInserted {
		s.created = true
	}
	return s.putInserted, nil
}

func (s *spyRepo) Update(_ context.Context, insight domain.Insight) error {
	if s.log != nil {
		s.log.add("repo.Update")
	}
	s.gotUpdateInsight = insight
	return s.updateErr
}

func (s *spyRepo) ListByTenantID(_ context.Context, _, tag string) ([]domain.Insight, error) {
	s.listCalled = true
	s.gotListTag = tag
	return s.listByTenantIDInsights, nil
}

func (s *spyRepo) ListByTag(_ context.Context, _, _ string) ([]domain.TagMembership, error) {
	return []domain.TagMembership{}, nil
}

func (s *spyRepo) ListTags(_ context.Context, _ string) ([]domain.TagSummary, error) {
	return []domain.TagSummary{}, nil
}

type spyEnrichmentClient struct {
	log *callLog

	enrichErr    error
	returnEnrich domain.Enrichment
	gotText      string
}

func (s *spyEnrichmentClient) Enrich(_ context.Context, text string) (domain.Enrichment, error) {
	if s.log != nil {
		s.log.add("llm.Enrich")
	}
	s.gotText = text
	if s.enrichErr != nil {
		return domain.Enrichment{}, s.enrichErr
	}
	return s.returnEnrich, nil
}

type spyDomainEventPublisher struct {
	log *callLog

	failEventType domain.EventType
	failErr       error

	published []domain.DomainEvent
}

func (s *spyDomainEventPublisher) Publish(_ context.Context, event domain.DomainEvent) error {
	if s.log != nil {
		s.log.add("events.Publish:" + string(event.EventType))
	}
	s.published = append(s.published, event)
	if s.failErr != nil && event.EventType == s.failEventType {
		return s.failErr
	}
	return nil
}

func makeInsight(id string) domain.Insight {
	return domain.Insight{
		ID:       id,
		TenantID: "t-1",
		Source:   "readwise",
		Text:     "hello world",
	}
}

func TestService_Process_HardGuard_EmptyID(t *testing.T) {
	log := &callLog{}
	repo := &spyRepo{log: log}
	spy := &spyEnrichmentClient{log: log}
	pub := &spyDomainEventPublisher{log: log}
	svc := NewService(repo, llm.NewService(spy), pub)

	_, err := svc.Process(context.Background(), makeInsight(""))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.As(err, &apperr.PermanentError{}) {
		t.Fatalf("expected PermanentError, got %v", err)
	}
	if len(log.entries) != 0 {
		t.Fatalf("expected no calls, got %v", log.entries)
	}
}

func TestService_Process_WhenNew_PutThenEnrichThenUpdate_StrictOrder(t *testing.T) {
	log := &callLog{}
	repo := &spyRepo{log: log, putInserted: true}
	spy := &spyEnrichmentClient{log: log}
	pub := &spyDomainEventPublisher{log: log}
	svc := NewService(repo, llm.NewService(spy), pub)

	_, err := svc.Process(context.Background(), makeInsight("idk-123"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	want := []string{"repo.CreateIfAbsent", "events.Publish:InsightCreated", "llm.Enrich", "repo.Update", "events.Publish:InsightEnriched"}
	if len(log.entries) != len(want) {
		t.Fatalf("expected calls=%v, got %v", want, log.entries)
	}
	for i := range want {
		if log.entries[i] != want[i] {
			t.Fatalf("expected calls=%v, got %v", want, log.entries)
		}
	}
}

func TestService_Process_WhenDuplicate_SkipsEnrichAndUpdate(t *testing.T) {
	log := &callLog{}
	repo := &spyRepo{log: log, putInserted: false}
	spy := &spyEnrichmentClient{log: log}
	pub := &spyDomainEventPublisher{log: log}
	svc := NewService(repo, llm.NewService(spy), pub)

	res, err := svc.Process(context.Background(), makeInsight("idk-dup"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Inserted {
		t.Fatalf("expected Inserted=false for duplicate, got true")
	}

	want := []string{"repo.CreateIfAbsent"}
	if len(log.entries) != len(want) || log.entries[0] != want[0] {
		t.Fatalf("expected calls=%v, got %v", want, log.entries)
	}
}

func TestService_Process_WhenRepoPutFails_ReturnsError_SkipsEnrichAndUpdate(t *testing.T) {
	log := &callLog{}
	putErr := errors.New("put boom")
	repo := &spyRepo{log: log, putErr: putErr}
	spy := &spyEnrichmentClient{log: log}
	pub := &spyDomainEventPublisher{log: log}
	svc := NewService(repo, llm.NewService(spy), pub)

	_, err := svc.Process(context.Background(), makeInsight("idk-puterr"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, putErr) {
		t.Fatalf("expected put error, got %v", err)
	}

	want := []string{"repo.CreateIfAbsent"}
	if len(log.entries) != len(want) || log.entries[0] != want[0] {
		t.Fatalf("expected calls=%v, got %v", want, log.entries)
	}
}

func TestService_Process_WhenEnrichFails_SoftFail_InsightStillInserted(t *testing.T) {
	log := &callLog{}
	repo := &spyRepo{log: log, putInserted: true}
	spy := &spyEnrichmentClient{log: log, enrichErr: errors.New("enrich boom")}
	pub := &spyDomainEventPublisher{log: log}
	svc := NewService(repo, llm.NewService(spy), pub)

	res, err := svc.Process(context.Background(), makeInsight("idk-enricherr"))
	if err != nil {
		t.Fatalf("expected no error (soft fail), got %v", err)
	}
	if !res.Inserted {
		t.Fatalf("expected Inserted=true even on enrichment failure, got false")
	}

	want := []string{"repo.CreateIfAbsent", "events.Publish:InsightCreated", "llm.Enrich"}
	if len(log.entries) != len(want) {
		t.Fatalf("expected calls=%v, got %v", want, log.entries)
	}
	for i := range want {
		if log.entries[i] != want[i] {
			t.Fatalf("expected calls=%v, got %v", want, log.entries)
		}
	}
	if len(pub.published) != 1 {
		t.Fatalf("expected InsightEnriched to never publish on soft-failed enrichment, got %v", pub.published)
	}
}

func TestService_Process_WhenUpdateFails_ReturnsError_AfterPutAndEnrich(t *testing.T) {
	log := &callLog{}
	updateErr := errors.New("update boom")
	repo := &spyRepo{log: log, putInserted: true, updateErr: updateErr}
	spy := &spyEnrichmentClient{log: log}
	pub := &spyDomainEventPublisher{log: log}
	svc := NewService(repo, llm.NewService(spy), pub)

	_, err := svc.Process(context.Background(), makeInsight("idk-updateerr"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, updateErr) {
		t.Fatalf("expected update error, got %v", err)
	}

	want := []string{"repo.CreateIfAbsent", "events.Publish:InsightCreated", "llm.Enrich", "repo.Update"}
	if len(log.entries) != len(want) {
		t.Fatalf("expected calls=%v, got %v", want, log.entries)
	}
	for i := range want {
		if log.entries[i] != want[i] {
			t.Fatalf("expected calls=%v, got %v", want, log.entries)
		}
	}
	if len(pub.published) != 1 {
		t.Fatalf("expected InsightEnriched to never publish when the update itself fails, got %v", pub.published)
	}
}

func TestService_Process_NilEnricher_SkipsEnrichAndUpdate(t *testing.T) {
	log := &callLog{}
	repo := &spyRepo{log: log, putInserted: true}
	pub := &spyDomainEventPublisher{log: log}
	svc := NewService(repo, nil, pub)

	res, err := svc.Process(context.Background(), makeInsight("idk-nilenr"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.Inserted {
		t.Fatalf("expected Inserted=true, got false")
	}

	want := []string{"repo.CreateIfAbsent", "events.Publish:InsightCreated"}
	if len(log.entries) != len(want) {
		t.Fatalf("expected calls=%v, got %v", want, log.entries)
	}
	for i := range want {
		if log.entries[i] != want[i] {
			t.Fatalf("expected calls=%v, got %v", want, log.entries)
		}
	}
}

func TestService_Process_PropagatesInsightToRepo(t *testing.T) {
	log := &callLog{}
	repo := &spyRepo{log: log, putInserted: true}
	spy := &spyEnrichmentClient{log: log}
	pub := &spyDomainEventPublisher{log: log}
	svc := NewService(repo, llm.NewService(spy), pub)

	_, err := svc.Process(context.Background(), makeInsight("idk-prop"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if repo.gotPutInsight.ID != "idk-prop" {
		t.Fatalf("expected id propagated into CreateIfAbsent, got %q", repo.gotPutInsight.ID)
	}
	if spy.gotText == "" {
		t.Fatalf("expected text to be sent to enrichment client")
	}
}

func TestService_Process_EnrichmentInput_IncludesNotes(t *testing.T) {
	log := &callLog{}
	repo := &spyRepo{log: log, putInserted: true}
	spy := &spyEnrichmentClient{log: log}
	pub := &spyDomainEventPublisher{log: log}
	svc := NewService(repo, llm.NewService(spy), pub)

	insight := makeInsight("idk-notes")
	insight.Notes = "reminds me of stoicism"

	_, err := svc.Process(context.Background(), insight)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if !strings.Contains(spy.gotText, insight.Text) || !strings.Contains(spy.gotText, insight.Notes) {
		t.Fatalf("expected enrichment input to include both text and notes, got %q", spy.gotText)
	}
}

func TestService_Process_UpdateReceivesEnrichmentFromLLM(t *testing.T) {
	log := &callLog{}
	repo := &spyRepo{log: log, putInserted: true}
	spy := &spyEnrichmentClient{
		log: log,
		returnEnrich: domain.Enrichment{
			Tags: []string{"learning", "growth"},
		},
	}
	pub := &spyDomainEventPublisher{log: log}
	svc := NewService(repo, llm.NewService(spy), pub)

	_, err := svc.Process(context.Background(), makeInsight("idk-enriched"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	got := repo.gotUpdateInsight.Enrichment
	if got == nil {
		t.Fatal("expected enrichment to be set on updated insight")
	}
	if len(got.Tags) != 2 || got.Tags[0] != "learning" || got.Tags[1] != "growth" {
		t.Fatalf("expected tags=[learning growth], got %v", got.Tags)
	}

	if len(pub.published) != 2 {
		t.Fatalf("expected 2 published events, got %v", pub.published)
	}
	enriched, ok := pub.published[1].Payload.(domain.InsightEnrichedPayload)
	if !ok || len(enriched.Tags) != 2 || enriched.Tags[0] != "learning" {
		t.Fatalf("expected InsightEnriched payload to carry the same tags, got %+v ok=%v", pub.published[1].Payload, ok)
	}
}

func TestService_Process_Redelivery_PublishesInsightCreatedOnlyOnce(t *testing.T) {
	log := &callLog{}
	repo := &spyRepo{log: log, putInserted: true}
	pub := &spyDomainEventPublisher{log: log}
	svc := NewService(repo, nil, pub)

	target := makeInsight("idk-once")
	if _, err := svc.Process(context.Background(), target); err != nil {
		t.Fatalf("unexpected err on first delivery: %v", err)
	}
	if _, err := svc.Process(context.Background(), target); err != nil {
		t.Fatalf("unexpected err on redelivery: %v", err)
	}

	created := 0
	for _, e := range pub.published {
		if e.EventType == domain.InsightCreated {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("expected exactly one InsightCreated event across redelivery, got %d (of %v)", created, pub.published)
	}
}

func TestService_Process_PublishFailureIsTransient(t *testing.T) {
	t.Run("InsightCreated publish failure", func(t *testing.T) {
		log := &callLog{}
		repo := &spyRepo{log: log, putInserted: true}
		spy := &spyEnrichmentClient{log: log}
		publishErr := errors.New("eventbridge boom")
		pub := &spyDomainEventPublisher{log: log, failEventType: domain.InsightCreated, failErr: publishErr}
		svc := NewService(repo, llm.NewService(spy), pub)

		_, err := svc.Process(context.Background(), makeInsight("idk-pub-created-fail"))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if errors.As(err, &apperr.PermanentError{}) {
			t.Fatalf("expected a transient error (not PermanentError, so SQS retries), got %v", err)
		}
		if !errors.Is(err, publishErr) {
			t.Fatalf("expected publish error, got %v", err)
		}

		want := []string{"repo.CreateIfAbsent", "events.Publish:InsightCreated"}
		if len(log.entries) != len(want) {
			t.Fatalf("expected calls=%v (enrichment should not run after a failed publish), got %v", want, log.entries)
		}
	})

	t.Run("InsightEnriched publish failure", func(t *testing.T) {
		log := &callLog{}
		repo := &spyRepo{log: log, putInserted: true}
		spy := &spyEnrichmentClient{log: log}
		publishErr := errors.New("eventbridge boom")
		pub := &spyDomainEventPublisher{log: log, failEventType: domain.InsightEnriched, failErr: publishErr}
		svc := NewService(repo, llm.NewService(spy), pub)

		_, err := svc.Process(context.Background(), makeInsight("idk-pub-enriched-fail"))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if errors.As(err, &apperr.PermanentError{}) {
			t.Fatalf("expected a transient error (not PermanentError, so SQS retries), got %v", err)
		}
		if !errors.Is(err, publishErr) {
			t.Fatalf("expected publish error, got %v", err)
		}

		want := []string{"repo.CreateIfAbsent", "events.Publish:InsightCreated", "llm.Enrich", "repo.Update", "events.Publish:InsightEnriched"}
		if len(log.entries) != len(want) {
			t.Fatalf("expected calls=%v, got %v", want, log.entries)
		}
	})
}

func TestService_ListByTenantID_NoTag_PassesThroughEmpty(t *testing.T) {
	repo := &spyRepo{}
	svc := NewService(repo, nil, &spyDomainEventPublisher{})

	if _, err := svc.ListByTenantID(context.Background(), "t-1", ""); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !repo.listCalled || repo.gotListTag != "" {
		t.Fatalf("expected repo called with empty tag, got called=%v tag=%q", repo.listCalled, repo.gotListTag)
	}
}

func TestService_ListByTenantID_DenormalizedTag_NormalizesBeforeQuery(t *testing.T) {
	repo := &spyRepo{}
	svc := NewService(repo, nil, &spyDomainEventPublisher{})

	if _, err := svc.ListByTenantID(context.Background(), "t-1", "Delegation"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if repo.gotListTag != "delegation" {
		t.Fatalf("expected normalized tag %q, got %q", "delegation", repo.gotListTag)
	}
}

func TestService_ListByTenantID_UnnormalizableTag_SkipsRepoReturnsEmpty(t *testing.T) {
	repo := &spyRepo{}
	svc := NewService(repo, nil, &spyDomainEventPublisher{})

	insights, err := svc.ListByTenantID(context.Background(), "t-1", "###")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(insights) != 0 {
		t.Fatalf("expected empty result, got %v", insights)
	}
	if repo.listCalled {
		t.Fatalf("expected repo not called for unnormalizable tag")
	}
}
