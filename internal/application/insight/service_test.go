package insight

import (
	"context"
	"errors"
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

	updateErr error

	gotPutInsight    domain.Insight
	gotUpdateInsight domain.Insight
}

func (s *spyRepo) CreateIfAbsent(_ context.Context, insight domain.Insight) (bool, error) {
	if s.log != nil {
		s.log.add("repo.CreateIfAbsent")
	}
	s.gotPutInsight = insight
	return s.putInserted, s.putErr
}

func (s *spyRepo) Update(_ context.Context, insight domain.Insight) error {
	if s.log != nil {
		s.log.add("repo.Update")
	}
	s.gotUpdateInsight = insight
	return s.updateErr
}

func (s *spyRepo) ListByTenantID(_ context.Context, _ string) ([]domain.Insight, error) {
	return []domain.Insight{}, nil
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
	svc := NewService(repo, llm.NewService(spy))

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
	svc := NewService(repo, llm.NewService(spy))

	_, err := svc.Process(context.Background(), makeInsight("idk-123"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	want := []string{"repo.CreateIfAbsent", "llm.Enrich", "repo.Update"}
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
	svc := NewService(repo, llm.NewService(spy))

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
	svc := NewService(repo, llm.NewService(spy))

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
	svc := NewService(repo, llm.NewService(spy))

	res, err := svc.Process(context.Background(), makeInsight("idk-enricherr"))
	if err != nil {
		t.Fatalf("expected no error (soft fail), got %v", err)
	}
	if !res.Inserted {
		t.Fatalf("expected Inserted=true even on enrichment failure, got false")
	}

	want := []string{"repo.CreateIfAbsent", "llm.Enrich"}
	if len(log.entries) != len(want) {
		t.Fatalf("expected calls=%v, got %v", want, log.entries)
	}
	for i := range want {
		if log.entries[i] != want[i] {
			t.Fatalf("expected calls=%v, got %v", want, log.entries)
		}
	}
}

func TestService_Process_WhenUpdateFails_ReturnsError_AfterPutAndEnrich(t *testing.T) {
	log := &callLog{}
	updateErr := errors.New("update boom")
	repo := &spyRepo{log: log, putInserted: true, updateErr: updateErr}
	spy := &spyEnrichmentClient{log: log}
	svc := NewService(repo, llm.NewService(spy))

	_, err := svc.Process(context.Background(), makeInsight("idk-updateerr"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, updateErr) {
		t.Fatalf("expected update error, got %v", err)
	}

	want := []string{"repo.CreateIfAbsent", "llm.Enrich", "repo.Update"}
	if len(log.entries) != len(want) {
		t.Fatalf("expected calls=%v, got %v", want, log.entries)
	}
	for i := range want {
		if log.entries[i] != want[i] {
			t.Fatalf("expected calls=%v, got %v", want, log.entries)
		}
	}
}

func TestService_Process_NilEnricher_SkipsEnrichAndUpdate(t *testing.T) {
	log := &callLog{}
	repo := &spyRepo{log: log, putInserted: true}
	svc := NewService(repo, nil)

	res, err := svc.Process(context.Background(), makeInsight("idk-nilenr"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.Inserted {
		t.Fatalf("expected Inserted=true, got false")
	}

	want := []string{"repo.CreateIfAbsent"}
	if len(log.entries) != len(want) || log.entries[0] != want[0] {
		t.Fatalf("expected calls=%v, got %v", want, log.entries)
	}
}

func TestService_Process_PropagatesInsightToRepo(t *testing.T) {
	log := &callLog{}
	repo := &spyRepo{log: log, putInserted: true}
	spy := &spyEnrichmentClient{log: log}
	svc := NewService(repo, llm.NewService(spy))

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

func TestService_Process_UpdateReceivesEnrichmentFromLLM(t *testing.T) {
	log := &callLog{}
	repo := &spyRepo{log: log, putInserted: true}
	spy := &spyEnrichmentClient{
		log: log,
		returnEnrich: domain.Enrichment{
			Summary:     "the core takeaway",
			Tags:        []string{"learning", "growth"},
			KeyQuestion: "What is the key lesson?",
		},
	}
	svc := NewService(repo, llm.NewService(spy))

	_, err := svc.Process(context.Background(), makeInsight("idk-enriched"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	got := repo.gotUpdateInsight.Enrichment
	if got == nil {
		t.Fatal("expected enrichment to be set on updated insight")
	}
	if got.Summary != "the core takeaway" {
		t.Fatalf("expected summary=%q, got %q", "the core takeaway", got.Summary)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "learning" || got.Tags[1] != "growth" {
		t.Fatalf("expected tags=[learning growth], got %v", got.Tags)
	}
	if got.KeyQuestion != "What is the key lesson?" {
		t.Fatalf("expected key_question=%q, got %q", "What is the key lesson?", got.KeyQuestion)
	}
}
