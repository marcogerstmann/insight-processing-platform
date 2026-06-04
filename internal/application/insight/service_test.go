package insight

import (
	"context"
	"errors"
	"testing"

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

type spyEnricher struct {
	log *callLog

	enrichErr error

	returnInsight domain.Insight
	gotInsight    domain.Insight
}

func (s *spyEnricher) Enrich(_ context.Context, insight domain.Insight) (domain.Insight, error) {
	if s.log != nil {
		s.log.add("enricher.Enrich")
	}
	s.gotInsight = insight
	if s.enrichErr != nil {
		return domain.Insight{}, s.enrichErr
	}
	if s.returnInsight.ID != "" {
		return s.returnInsight, nil
	}
	return insight, nil
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
	enr := &spyEnricher{log: log}
	svc := NewService(repo, enr)

	_, err := svc.Process(context.Background(), makeInsight(""))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, errMissingID) {
		t.Fatalf("expected errMissingID, got %v", err)
	}
	if len(log.entries) != 0 {
		t.Fatalf("expected no calls, got %v", log.entries)
	}
}

func TestService_Process_WhenNew_PutThenEnrichThenUpdate_StrictOrder(t *testing.T) {
	log := &callLog{}
	repo := &spyRepo{log: log, putInserted: true}
	enr := &spyEnricher{log: log}
	svc := NewService(repo, enr)

	_, err := svc.Process(context.Background(), makeInsight("idk-123"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	want := []string{"repo.CreateIfAbsent", "enricher.Enrich", "repo.Update"}
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
	enr := &spyEnricher{log: log}
	svc := NewService(repo, enr)

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
	enr := &spyEnricher{log: log}
	svc := NewService(repo, enr)

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

func TestService_Process_WhenEnrichFails_ReturnsError_DoesNotUpdate(t *testing.T) {
	log := &callLog{}
	enrichErr := errors.New("enrich boom")
	repo := &spyRepo{log: log, putInserted: true}
	enr := &spyEnricher{log: log, enrichErr: enrichErr}
	svc := NewService(repo, enr)

	_, err := svc.Process(context.Background(), makeInsight("idk-enricherr"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, enrichErr) {
		t.Fatalf("expected enrich error, got %v", err)
	}

	want := []string{"repo.CreateIfAbsent", "enricher.Enrich"}
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
	enr := &spyEnricher{log: log}
	svc := NewService(repo, enr)

	_, err := svc.Process(context.Background(), makeInsight("idk-updateerr"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, updateErr) {
		t.Fatalf("expected update error, got %v", err)
	}

	want := []string{"repo.CreateIfAbsent", "enricher.Enrich", "repo.Update"}
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
	enr := &spyEnricher{log: log}
	svc := NewService(repo, enr)

	_, err := svc.Process(context.Background(), makeInsight("idk-prop"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if repo.gotPutInsight.ID != "idk-prop" {
		t.Fatalf("expected id propagated into CreateIfAbsent, got %q", repo.gotPutInsight.ID)
	}
	if enr.gotInsight.ID != "idk-prop" {
		t.Fatalf("expected id propagated into enricher, got %q", enr.gotInsight.ID)
	}
}

func TestService_Process_UpdateReceivesEnrichedInsight_NotOriginal(t *testing.T) {
	log := &callLog{}
	repo := &spyRepo{log: log, putInserted: true}

	enriched := domain.Insight{
		ID:       "idk-enriched",
		TenantID: "t-1",
		Source:   "readwise",
		Text:     "enriched text",
	}

	enr := &spyEnricher{log: log, returnInsight: enriched}
	svc := NewService(repo, enr)

	_, err := svc.Process(context.Background(), makeInsight("idk-enriched"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if repo.gotUpdateInsight.Text != "enriched text" {
		t.Fatalf("expected update to receive enriched insight, got %q", repo.gotUpdateInsight.Text)
	}
}
